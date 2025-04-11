package messagerepo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v4"
	opensearchapi "github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/testcontainers/testcontainers-go"
	tcopensearch "github.com/testcontainers/testcontainers-go/modules/opensearch"
)

var (
	// Serialization/deserialization errors.
	ErrSerializeMessage    = errors.New("messagerepo: failed to serialize message")
	ErrDeserializeResponse = errors.New("messagerepo: failed to deserialize response")

	// Index-related errors.
	ErrIndexCreationFailed = errors.New("messagerepo: failed to create index")
	ErrIndexCheckFailed    = errors.New("messagerepo: failed to check index existence")

	// Document retrieval errors.
	ErrMessageNotFound = errors.New("messagerepo: message not found")

	// Operation errors.
	ErrSearchFailed = errors.New("messagerepo: search failed")
	ErrUpdateFailed = errors.New("messagerepo: update failed")
	ErrDeleteFailed = errors.New("messagerepo: delete failed")
)

type Message struct {
	ID          string    `json:"id"`
	MessageID   string    `json:"messageId"`
	SpecVersion string    `json:"specversion"`
	Type        string    `json:"type"`
	Time        time.Time `json:"time"`
	Subject     string    `json:"subject"`
	Source      string    `json:"source"`
	Data        string    `json:"data"`
	ReceivedAt  time.Time `json:"receivedAt"`
}

// MessageUpdate defines the updatable fields for a Message.
// Fields that are nil will be omitted in the update.
type MessageUpdate struct {
	Subject *string `json:"subject,omitempty"`
	Data    *string `json:"data,omitempty"`
}

// Store defines the operations for persisting and retrieving chat messages.
// Implementations should adhere to these behaviors without exposing internal details.
type Store interface {
	// Save persists a chat message.
	Save(ctx context.Context, msg Message) error

	// SearchByID retrieves a chat message by its unique identifier.
	SearchByID(ctx context.Context, id string) (Message, error)

	// Search queries chat messages based on various criteria.
	// Parameters:
	//   - query: text to match within the message subject (if empty, all messages are returned)
	//   - startTime, endTime: optional time range filters for the message timestamp
	//   - filterSource, filterType: optional exact-match filters for source and type fields
	//   - page, pageSize: pagination parameters (page is 1-based; if pageSize==0 then pagination is ignored)
	//   - sortField, sortOrder: sorting parameters (defaults are applied if empty)
	//
	// Returns:
	//   - a slice of messages matching the query
	//   - the total number of matching messages
	//   - the query execution duration in milliseconds
	Search(ctx context.Context, query string, startTime, endTime *time.Time, filterSource, filterType string, page, pageSize int, sortField, sortOrder string) ([]Message, int64, int64, error)

	// Update modifies specified fields of a stored chat message.
	Update(ctx context.Context, id string, update MessageUpdate) error

	// Delete removes a chat message from storage.
	Delete(ctx context.Context, id string) error
}

var _ Store = &Index{}

// Index is a concrete implementation of Courier using an underlying document store.
// It wraps an OpenSearch client to perform document operations.
type Index struct {
	*opensearch.Client
	IndexName string
}

// New creates a new Courier instance by initializing the underlying storage client.
// It performs an initial connection test and ensures the necessary index is available.
func New(ctx context.Context, opensearchURL, indexName string, responseTimeout time.Duration) (Store, error) {
	log.Println("Initializing storage client")
	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{opensearchURL},
		Transport: &http.Transport{
			ResponseHeaderTimeout: responseTimeout,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}
	idx := &Index{
		Client:    client,
		IndexName: indexName,
	}
	log.Println("Ensuring storage index exists")
	if err = idx.ensureIndex(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIndexCreationFailed, err)
	}
	log.Println("Storage index ready")
	return idx, nil
}

// ensureIndex verifies that the storage index exists and creates it if necessary.
// This internal method sets up the index with a predefined mapping.
func (idx *Index) ensureIndex(ctx context.Context) error {
	existsRes, err := idx.Client.Do(ctx, opensearchapi.IndexTemplateExistsReq{IndexTemplate: idx.IndexName}, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIndexCheckFailed, err)
	}

	defer existsRes.Body.Close()

	if existsRes.StatusCode == http.StatusOK {
		log.Printf("Index %s already exists", idx.IndexName)
		return nil
	}

	// Predefined mapping and settings.
	mapping := fmt.Sprintf(`{
	  "index_patterns": ["%s-*"],
	  "template": {
	    "settings": {
	      "number_of_shards": 1,
	      "number_of_replicas": 1,
	      "refresh_interval": "1s"
	    },
	    "mappings": {
	      "properties": {
	        "id":          { "type": "keyword" },
	        "messageId":   { "type": "keyword" },
	        "specversion": { "type": "keyword" },
	        "type":        { "type": "keyword" },
	        "time":        { "type": "date", "format": "strict_date_optional_time||epoch_millis" },
	        "subject":     { "type": "text" },
	        "source":      { "type": "keyword" },
	        "data":        { "type": "text" },
	        "receivedAt":  { "type": "date", "format": "strict_date_optional_time||epoch_millis" }
	      }
	    }
	  }
	}`, idx.IndexName)

	// Create the index using the predefined mapping.
	createRes, err := idx.Client.Do(ctx, opensearchapi.IndexTemplateCreateReq{
		IndexTemplate: idx.IndexName,
		Body:          bytes.NewReader([]byte(mapping)),
	}, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrIndexCreationFailed, err)
	}
	defer createRes.Body.Close()

	if createRes.StatusCode != http.StatusOK && createRes.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createRes.Body)
		return fmt.Errorf("%w: failed to create index, response code: %d, body: %s", ErrIndexCreationFailed, createRes.StatusCode, string(body))
	}
	log.Printf("Index %s created successfully", idx.IndexName)
	return nil
}

// Save persists a single chat message.
func (idx *Index) Save(ctx context.Context, msg Message) error {
	jsonBody, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSerializeMessage, err)
	}

	if msg.ID == "" {
		return fmt.Errorf("%w: message ID is required", ErrSerializeMessage)
	}

	res, err := idx.Client.Do(ctx, opensearchapi.IndexReq{
		Index:      idx.IndexName,
		DocumentID: msg.ID,
		Body:       bytes.NewReader(jsonBody),
		Params: opensearchapi.IndexParams{
			Refresh: "true",
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("error storing message: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to store message, response code: %d", res.StatusCode)
	}
	log.Printf("Message with ID %s stored successfully", msg.ID)
	return nil
}

// SearchByID retrieves a chat message by its unique identifier.
func (idx *Index) SearchByID(ctx context.Context, id string) (Message, error) {
	var msg Message
	res, err := idx.Client.Do(ctx, opensearchapi.DocumentGetReq{
		Index:      idx.IndexName,
		DocumentID: id,
	}, nil)
	if err != nil {
		return msg, fmt.Errorf("error retrieving message by ID: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return msg, fmt.Errorf("%w: message with ID %s not found", ErrMessageNotFound, id)
	}
	if res.StatusCode != http.StatusOK {
		return msg, fmt.Errorf("unexpected response status: %d", res.StatusCode)
	}

	var response struct {
		Source Message `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return msg, fmt.Errorf("%w: %v", ErrDeserializeResponse, err)
	}

	return response.Source, nil
}

func (idx *Index) Search(ctx context.Context, query string, startTime, endTime *time.Time, filterSource, filterType string, page, pageSize int, sortField, sortOrder string) ([]Message, int64, int64, error) {
	// Build query components.
	mustClauses := []map[string]any{}
	if query != "" {
		mustClauses = append(mustClauses, map[string]any{
			"match": map[string]any{
				"subject": query,
			},
		})
	}

	filters := []map[string]any{}
	if startTime != nil || endTime != nil {
		rangeQuery := map[string]any{}
		if startTime != nil {
			rangeQuery["gte"] = startTime
		}
		if endTime != nil {
			rangeQuery["lte"] = endTime
		}
		filters = append(filters, map[string]any{
			"range": map[string]any{
				"time": rangeQuery,
			},
		})
	}
	if filterSource != "" {
		filters = append(filters, map[string]any{
			"term": map[string]any{
				"source": filterSource,
			},
		})
	}
	if filterType != "" {
		filters = append(filters, map[string]any{
			"term": map[string]any{
				"type": filterType,
			},
		})
	}

	var boolQuery any
	if len(mustClauses) == 0 && len(filters) == 0 {
		boolQuery = map[string]any{"match_all": map[string]any{}}
	} else {
		boolMap := map[string]any{}
		if len(mustClauses) > 0 {
			boolMap["must"] = mustClauses
		}
		if len(filters) > 0 {
			boolMap["filter"] = filters
		}
		boolQuery = map[string]any{"bool": boolMap}
	}

	queryBody := map[string]any{"query": boolQuery}
	if pageSize > 0 && page > 0 {
		from := (page - 1) * pageSize
		queryBody["from"] = from
		queryBody["size"] = pageSize
	}
	if sortField == "" {
		sortField = "time"
	}
	if sortOrder == "" {
		sortOrder = "asc"
	}
	queryBody["sort"] = []any{
		map[string]any{
			sortField: map[string]any{
				"order": sortOrder,
			},
		},
	}

	body, err := json.Marshal(queryBody)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrSerializeMessage, err)
	}

	res, err := idx.Client.Do(ctx, opensearchapi.SearchReq{
		Indices: []string{idx.IndexName},
		Body:    bytes.NewReader(body),
	}, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrSearchFailed, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		respBody, _ := io.ReadAll(res.Body)
		return nil, 0, 0, fmt.Errorf("%w: search failed (%d): %s", ErrSearchFailed, res.StatusCode, string(respBody))
	}

	var response struct {
		Took int64 `json:"took"`
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source Message `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrDeserializeResponse, err)
	}

	totalHits := response.Hits.Total.Value
	var messages []Message
	for _, hit := range response.Hits.Hits {
		messages = append(messages, hit.Source)
	}

	return messages, totalHits, response.Took, nil
}

// Update modifies specific fields of an existing chat message.
func (idx *Index) Update(ctx context.Context, id string, update MessageUpdate) error {
	updateBody, err := json.Marshal(map[string]any{"doc": update})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSerializeMessage, err)
	}

	res, err := idx.Client.Do(ctx, opensearchapi.UpdateReq{
		Index:      idx.IndexName,
		DocumentID: id,
		Body:       bytes.NewReader(updateBody),
	}, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUpdateFailed, err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%w: update failed (%d): %s", ErrUpdateFailed, res.StatusCode, string(body))
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: failed to update message, response code: %d", ErrUpdateFailed, res.StatusCode)
	}
	log.Printf("Message with ID %s updated successfully", id)
	return nil
}

// Delete removes a chat message from storage.
func (idx *Index) Delete(ctx context.Context, id string) error {
	res, err := idx.Client.Do(ctx, opensearchapi.DocumentDeleteReq{
		Index:      idx.IndexName,
		DocumentID: id,
		Params: opensearchapi.DocumentDeleteParams{
			Refresh: "true",
		},
	}, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDeleteFailed, err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("%w: delete failed (%d): %s", ErrDeleteFailed, res.StatusCode, string(body))
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: failed to delete message, response code: %d", ErrDeleteFailed, res.StatusCode)
	}
	log.Printf("Message with ID %s deleted successfully", id)
	return nil
}

func quiet() func() {
	null, _ := os.Open(os.DevNull)
	sout := os.Stdout
	serr := os.Stderr
	os.Stdout = null
	os.Stderr = null
	log.SetOutput(null)
	return func() {
		defer null.Close()
		os.Stdout = sout
		os.Stderr = serr
		log.SetOutput(os.Stderr)
	}
}

// NewTestStore initializes an Index instance backed by a test container.
// This is intended for integration testing purposes.
func NewTestStore(t *testing.T) (Store, func(), error) {
	t.Helper()
	unquiet := quiet()
	t.Cleanup(unquiet)
	ctx := context.TODO()
	fmt.Println("Starting test storage container...")
	ctr, err := tcopensearch.Run(ctx, "opensearchproject/opensearch:2.11.1")
	if err != nil {
		return nil, func() {}, err
	}
	cleanup := func() {
		fmt.Println("Cleaning up container...")
		testcontainers.CleanupContainer(t, ctr)
	}
	address, err := ctr.Address(ctx)
	if err != nil {
		return nil, cleanup, err
	}
	fmt.Printf("Test storage container address: %s\n", address)

	courier, err := New(ctx, address, "test", 10*time.Second)
	if err != nil {
		return nil, cleanup, err
	}
	return courier, cleanup, nil
}
