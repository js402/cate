package serverapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/contenox/contenox/core/llmrepo"
	"github.com/contenox/contenox/core/llmresolver"
	"github.com/contenox/contenox/core/modelprovider"
	"github.com/contenox/contenox/core/serverapi"
	"github.com/contenox/contenox/core/serverapi/dispatchapi"
	"github.com/contenox/contenox/core/serverapi/filesapi"
	"github.com/contenox/contenox/core/serverapi/indexapi"
	"github.com/contenox/contenox/core/serverapi/usersapi"
	"github.com/contenox/contenox/core/serverops"
	"github.com/contenox/contenox/core/serverops/store"
	"github.com/contenox/contenox/core/serverops/vectors"
	"github.com/contenox/contenox/core/services/dispatchservice"
	"github.com/contenox/contenox/core/services/fileservice"
	"github.com/contenox/contenox/core/services/indexservice"
	"github.com/contenox/contenox/core/services/testingsetup"
	"github.com/contenox/contenox/core/services/userservice"
	"github.com/contenox/contenox/libs/libauth"
	"github.com/contenox/contenox/libs/libtestenv"
	"github.com/stretchr/testify/require"
)

func TestWorkerPipe(t *testing.T) {
	if os.Getenv("SMOKETESTS") == "" {
		t.Skip("Set env SMOKETESTS to true to run this test")
	}
	port := rand.Intn(16383) + 49152
	config := &serverops.Config{
		JWTExpiry:       "1h",
		JWTSecret:       "securecryptngkeysecurecryptngkey",
		EncryptionKey:   "securecryptngkeysecurecryptngkey",
		SigningKey:      "securecryptngkeysecurecryptngkey",
		EmbedModel:      "nomic-embed-text:latest",
		TasksModel:      "qwen2.5:0.5b",
		SecurityEnabled: "true",
	}

	ctx, state, dbInstance, cleanup, err := testingsetup.SetupTestEnvironment(config)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}
	embedder, err := llmrepo.NewEmbedder(ctx, config, dbInstance, state)
	if err != nil {
		log.Fatalf("initializing embedding pool failed: %v", err)
	}
	execRepo, err := llmrepo.NewExecRepo(ctx, config, dbInstance, state)
	if err != nil {
		log.Fatalf("initializing exec repo failed: %v", err)
	}
	uri, _, cleanup2, err := vectors.SetupLocalInstance(ctx, "../../")
	defer cleanup2()
	if err != nil {
		t.Fatal(err)
	}
	config.VectorStoreURL = uri
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		// OK
	})

	fileService := fileservice.New(dbInstance, config)
	fileService = fileservice.WithActivityTracker(fileService, fileservice.NewFileVectorizationJobCreator(dbInstance))
	filesapi.AddFileRoutes(mux, config, fileService)

	vectorStore, cleanup4, err := vectors.New(ctx, config.VectorStoreURL, vectors.Args{
		Timeout: 1 * time.Second,
		SearchArgs: vectors.SearchArgs{
			Epsilon: 0.1,
			Radius:  -1,
		},
	})
	if err != nil {
		log.Fatalf("initializing vector store failed: %v", err)
	}
	defer cleanup4()
	indexService := indexservice.New(ctx, embedder, execRepo, vectorStore, dbInstance)
	indexapi.AddIndexRoutes(mux, config, indexService)

	userService := userservice.New(dbInstance, config)
	res, err := userService.Register(ctx, userservice.CreateUserRequest{
		Email:        serverops.DefaultAdminUser,
		FriendlyName: "Admin",
		Password:     "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	usersapi.AddAuthRoutes(mux, userService)
	ctx = context.WithValue(ctx, libauth.ContextTokenKey, res.Token)

	require.Eventually(t, func() bool {
		currentState := state.Get(ctx)
		r, err := json.Marshal(currentState)
		if err != nil {
			t.Logf("error marshaling state: %v", err)
			return false
		}
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, r); err != nil {
			t.Logf("error compacting JSON: %v", err)
			return false
		}
		return strings.Contains(string(r), `"name":"nomic-embed-text:latest"`)
	}, 2*time.Minute, 100*time.Millisecond)

	require.Eventually(t, func() bool {
		currentState := state.Get(ctx)
		r, err := json.Marshal(currentState)
		if err != nil {
			t.Logf("error marshaling state: %v", err)
			return false
		}
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, r); err != nil {
			t.Logf("error compacting JSON: %v", err)
			return false
		}
		return strings.Contains(string(r), `"name":"qwen2.5:0.5b"`)
	}, 2*time.Minute, 100*time.Millisecond)
	runtime := state.Get(ctx)
	url := ""
	backendID := ""
	found := false
	foundExecModel := false
	for _, runtimeState := range runtime {
		url = runtimeState.Backend.BaseURL
		backendID = runtimeState.Backend.ID
		for _, lmr := range runtimeState.PulledModels {
			if lmr.Model == "nomic-embed-text:latest" {
				found = true
			}
			if lmr.Model == "qwen2.5:0.5b" {
				foundExecModel = true
			}
		}
		if found && foundExecModel {
			break
		}
	}
	if !found {
		t.Fatalf("nomic-embed-text:latest not found")
	}
	if !foundExecModel {
		t.Fatalf("qwen2.5:0.5b not found")
	}
	_ = url
	err = store.New(dbInstance.WithoutTransaction()).AssignBackendToPool(ctx, serverops.EmbedPoolID, backendID)
	if err != nil {
		t.Fatalf("failed to assign backend to pool: %v", err)
	}
	// sanity check
	client, err := llmresolver.Embed(ctx, llmresolver.EmbedRequest{
		ModelName: "nomic-embed-text:latest",
	}, modelprovider.ModelProviderAdapter(ctx, state.Get(ctx)), llmresolver.Randomly)
	if err != nil {
		t.Fatalf("failed to resolve embed: %v", err)
	}
	// sanity-check 2
	backends, err := store.New(dbInstance.WithoutTransaction()).ListBackendsForPool(ctx, serverops.EmbedPoolID)
	if err != nil {
		t.Fatalf("failed to list backends for pool: %v", err)
	}
	found2 := false
	for _, backend2 := range backends {
		found2 = backend2.ID == backendID
		if found2 {
			break
		}
	}
	if !found2 {
		t.Fatalf("backend not found in pool")
	}
	dispatcher := dispatchservice.New(dbInstance, config)
	dispatchapi.AddDispatchRoutes(mux, config, dispatcher)
	handler := serverapi.JWTMiddleware(config, mux)
	go func() {
		if err := http.ListenAndServe("0.0.0.0:"+fmt.Sprint(port), handler); err != nil {
			log.Fatal(err)
		}
	}()

	// ensure embedder is ready
	embedderProvider, err := embedder.GetProvider(ctx)
	if err != nil {
		t.Fatalf("failed to get embedder provider: %v", err)
	}
	if !embedderProvider.CanEmbed() {
		t.Fatalf("embedder not ready")
	}
	file := &fileservice.File{
		Name:        "updated.txt",
		ContentType: "text/plain; charset=utf-8",
		Data:        []byte("some demo text to be embedded"),
	}
	vectorData, err := client.Embed(ctx, string(file.Data))
	if err != nil {
		t.Fatalf("failed to embed file: %v", err)
	}
	vectorData32 := make([]float32, len(vectorData))

	// Iterate and cast each element
	for i, v := range vectorData {
		vectorData32[i] = float32(v)
	}

	t.Logf("Dimension of query vector generated in test: %d", len(vectorData32))
	// sanity-check 3
	require.Equal(t, 768, len(vectorData32), "Query vector dimension mismatch")
	t.Run("create a file should trigger vectorization", func(t *testing.T) {
		file, err = fileService.CreateFile(ctx, file)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		jobs, err := dispatcher.PendingJobs(ctx, nil)
		require.NoError(t, err, "failed to get pending jobs")
		for i, j := range jobs {
			t.Log(fmt.Sprintf("JOB %d: %s %v %v", i, j.TaskType, j.ID, j.RetryCount))
		}
		require.GreaterOrEqual(t, len(jobs), 1, "expected 1 pending job")
		require.Equal(t, "vectorize_text/plain; charset=utf-8", jobs[0].TaskType, "expected plaintext job")
		workerContainer, cleanup3, err := libtestenv.SetupLocalWorkerInstance(ctx, libtestenv.WorkerConfig{
			APIBaseURL:                  fmt.Sprintf("http://172.17.0.1:%d", port),
			WorkerEmail:                 serverops.DefaultAdminUser,
			WorkerPassword:              "test",
			WorkerLeaserID:              "my-worker-1",
			WorkerLeaseDurationSeconds:  2,
			WorkerRequestTimeoutSeconds: 2,
			WorkerType:                  "plaintext",
		})
		defer cleanup3()
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Second * 3)
		readCloser, err := workerContainer.Logs(ctx)
		require.NoError(t, err, "failed to get worker logs stream")
		defer readCloser.Close()

		logBytes, err := io.ReadAll(readCloser)
		if err != nil && err != io.EOF {
			t.Logf("Warning: failed to read all worker logs: %v", err)
		}
		t.Logf("WORKER LOGS:\n%s\n--- END WORKER LOGS ---", string(logBytes))
		jobs, err = dispatcher.PendingJobs(ctx, nil)
		found := -1
		for i, j := range jobs {
			if j.TaskType == "vectorize_text/plain; charset=utf-8" {
				found = i
			}
			t.Log(fmt.Sprintf("JOB %d: %s %v %v", i, j.TaskType, j.ID, j.RetryCount))
		}
		errText := ""
		if found != -1 {
			errText = fmt.Sprintf("expected 0 pending job for vectorize_text %v %v", *&jobs[found].RetryCount, *jobs[found])
		}
		require.Equal(t, -1, found, errText)

		results, err := vectorStore.Search(ctx, vectorData32, 10, 1, nil) // prior 10
		if err != nil {
			t.Fatalf("failed to search vector store: %v", err)
		}
		if len(results) == 0 {
			t.Fatalf("no results found")
		}
		if len(results) < 1 {
			t.Fatalf("expected at least one vector, got %d", len(results))
		}
		chunk, err := store.New(dbInstance.WithoutTransaction()).GetChunkIndexByID(ctx, results[0].ID)
		if err != nil {
			t.Fatalf("failed to get chunk index by ID: %v", err)
		}
		if chunk.ResourceID != file.ID {
			t.Fatalf("expected file ID %s, got %s", file.ID, chunk.ResourceID)
		}
		resp, err := indexService.Search(ctx, &indexservice.SearchRequest{
			Query: "give me the file with the demo text",
			TopK:  10,
		})
		if err != nil {
			t.Fatal(err)
		}
		foundID := false
		for _, sr := range resp.Results {
			if sr.ID == file.ID {
				foundID = true
			}
		}
		if !foundID {
			t.Fatal("file was not found")
		}
	})
}
