// runtimestate implements the core logic for reconciling the declared state
// of LLM backends (from dbInstance) with their actual observed state.
// It provides the functionality for synchronizing models and processing downloads,
// intended to be executed repeatedly within background tasks managed externally.
// TODO: rewire for new pool feature
package runtimestate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/js402/cate/libs/libbus"
	"github.com/js402/cate/libs/libdb"
	"github.com/js402/cate/serverops/store"
	"github.com/ollama/ollama/api"
)

// LLMState represents the observed state of a single LLM backend.
type LLMState struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Models       []string                `json:"models"`
	PulledModels []api.ListModelResponse `json:"pulledModels"`
	Backend      store.Backend           `json:"backend"`
	// Error stores a description of the last encountered error when
	// interacting with or reconciling this backend's state, if any.
	Error string `json:"error,omitempty"`
}

// State manages the overall runtime status of multiple LLM backends.
// It orchestrates the synchronization between the desired configuration
// and the actual state of the backends, including providing the mechanism
// for model downloads via the dwqueue component.
type State struct {
	dbInstance libdb.DBManager
	state      sync.Map
	psInstance libbus.Messenger
	dwQueue    dwqueue
	withPools  bool
}

type Option func(*State)

func WithPools() Option {
	return func(s *State) {
		s.withPools = true
	}
}

// New creates and initializes a new State manager.
// It requires a database manager (dbInstance) to load the desired configurations
// and a messenger instance (psInstance) for event handling and progress updates.
// Options allow enabling experimental features like pool-based reconciliation.
// Returns an initialized State ready for use.
func New(ctx context.Context, dbInstance libdb.DBManager, psInstance libbus.Messenger, options ...Option) (*State, error) {
	s := &State{
		dbInstance: dbInstance,
		state:      sync.Map{},
		dwQueue:    dwqueue{dbInstance: dbInstance},
		psInstance: psInstance,
	}

	// Apply options to configure the State instance
	for _, option := range options {
		option(s)
	}
	return s, nil
}

// RunBackendCycle performs a single reconciliation check for all configured LLM backends.
// It compares the desired state (from configuration) with the observed state
// (by communicating with the backends) and schedules necessary actions,
// such as queuing model downloads or removals, to align them.
// This method should be called periodically in a background process.
// DESIGN NOTE: This method executes one complete reconciliation cycle and then returns.
// It does not manage its own background execution (e.g., via internal goroutines or timers).
// This deliberate design choice delegates execution management (scheduling, concurrency control,
// lifecycle via context, error handling, circuit breaking, etc.) entirely to the caller.
//
// Consequently, this method should be called periodically by an external process
// responsible for its scheduling and lifecycle.
// When the pool feature is enabled via WithPools option, it uses pool-aware reconciliation.
func (s *State) RunBackendCycle(ctx context.Context) error {
	if s.withPools {
		return s.syncBackendsWithPools(ctx)
	}
	return s.syncBackends(ctx)
}

// RunDownloadCycle processes a single pending model download operation, if one exists.
// It retrieves the next download task, executes the download while providing
// progress updates, and handles potential cancellation requests.
// If no download tasks are queued, it returns nil immediately.
// This method should be called periodically in a background process to
// drain the download queue.
// DESIGN NOTE: this method performs one unit of work
// and returns. The caller is responsible for the execution loop, allowing
// flexible integration with task management strategies.
//
// This method should be called periodically by an external process to
// drain the download queue.
func (s *State) RunDownloadCycle(ctx context.Context) error {
	item, err := s.dwQueue.pop(ctx)
	if err != nil {
		if err == libdb.ErrNotFound {
			return nil
		}
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // clean up the context when done

	done := make(chan struct{})

	ch := make(chan []byte, 16)
	sub, err := s.psInstance.Stream(ctx, "queue_cancel", ch)
	if err != nil {
		log.Println("Error subscribing to queue_cancel:", err)
		return nil
	}
	go func() {
		defer func() {
			sub.Unsubscribe()
			close(done)
		}()
		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				var queueItem store.Job
				if err := json.Unmarshal(data, &queueItem); err != nil {
					log.Println("Error unmarshalling cancel message:", err)
					continue
				}
				// Check if the cancellation request matches the current download task.
				// Rationale: Matching logic based on URL to target a specific backend
				// or Model ID to purge a model from all backends, if it is currently downloading.
				if queueItem.ID == item.URL || queueItem.ID == item.Model {
					cancel()
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	log.Printf("Processing download job: %+v", item)
	err = s.dwQueue.downloadModel(ctx, *item, func(status store.Status) error {
		// log.Printf("Download progress for model %s: %+v", item.Model, status)
		message, _ := json.Marshal(status)
		return s.psInstance.Publish(ctx, "model_download", message)
	})

	if err != nil {
		return fmt.Errorf("failed downloading model %s: %w", item.Model, err)
	}

	cancel()
	<-done

	return nil
}

// Get returns a copy of the current observed state for all backends.
// This provides a safe snapshot for reading state without risking modification
// of the internal structures.
func (s *State) Get(ctx context.Context) map[string]LLMState {
	state := map[string]LLMState{}
	s.state.Range(func(key, value any) bool {
		backend, ok := value.(*LLMState)
		if !ok {
			log.Printf("invalid type in state: %T", value)
			return true
		}
		var backendCopy LLMState
		raw, err := json.Marshal(backend)
		if err != nil {
			log.Printf("failed to marshal backend: %v", err)
		}
		err = json.Unmarshal(raw, &backendCopy)
		if err != nil {
			log.Printf("failed to unmarshal backend: %v", err)
		}
		state[backend.ID] = backendCopy
		return true
	})
	return state
}

// Helper method to process backends and collect their IDs
func (s *State) processBackends(ctx context.Context, backends []*store.Backend, models []*store.Model, currentIDs map[string]struct{}) {
	for _, backend := range backends {
		currentIDs[backend.ID] = struct{}{}
		s.processBackend(ctx, backend, models)
	}
}

// cleanupStaleBackends removes state entries for backends not present in currentIDs.
// It performs type checking on state keys and logs errors for invalid key types.
// This centralizes the state cleanup logic used by all reconciliation flows.
func (s *State) cleanupStaleBackends(currentIDs map[string]struct{}) error {
	var err error
	s.state.Range(func(key, value any) bool {
		id, ok := key.(string)
		if !ok {
			err = fmt.Errorf("BUG: invalid key type: %T %v", key, key)
			log.Printf("BUG: %v", err)
			return true
		}
		if _, exists := currentIDs[id]; !exists {
			s.state.Delete(id)
		}
		return true
	})
	return err
}

// syncBackendsWithPools is the pool-aware reconciliation logic called by RunBackendCycle.
// It:
//  1. Fetches all configured pools from the database
//  2. For each pool:
//     a. Retrieves associated backends
//     b. Fetches pool-specific models
//     c. Processes each backend with its pool's models
//  3. After processing all pools:
//     a. Performs global cleanup of state entries for backends not found in any pool
//
// This fixed version aggregates backend IDs across all pools before cleanup to prevent
// premature deletion of valid cross-pool backends.
func (s *State) syncBackendsWithPools(ctx context.Context) error {
	tx := s.dbInstance.WithoutTransaction()
	store := store.New(tx)

	pools, err := store.ListPools(ctx)
	if err != nil {
		return fmt.Errorf("fetching pools: %v", err)
	}

	currentIDs := make(map[string]struct{}) // Shared across all pools

	for _, pool := range pools {
		backends, err := store.ListBackendsForPool(ctx, pool.ID)
		if err != nil {
			return fmt.Errorf("fetching backends for pool %s: %v", pool.ID, err)
		}

		models, err := store.ListModelsForPool(ctx, pool.ID)
		if err != nil {
			return fmt.Errorf("fetching models: %v", err)
		}

		// Process pool's backends and accumulate IDs
		s.processBackends(ctx, backends, models, currentIDs)
	}

	// Single cleanup after processing all pools
	return s.cleanupStaleBackends(currentIDs)
}

// syncBackends is the global reconciliation logic called by RunBackendCycle.
// It:
// 1. Fetches all configured backends from the database
// 2. Retrieves all models regardless of pool association
// 3. Processes each backend with the full model list
// 4. Cleans up state entries for backends no longer present in the database
// This version uses the shared helper methods but maintains its original non-pool
// behavior by operating on the global backend/model lists.
func (s *State) syncBackends(ctx context.Context) error {
	tx := s.dbInstance.WithoutTransaction()
	store := store.New(tx)

	backends, err := store.ListBackends(ctx)
	if err != nil {
		return fmt.Errorf("fetching backends: %v", err)
	}

	models, err := store.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("fetching models: %v", err)
	}

	currentIDs := make(map[string]struct{})
	s.processBackends(ctx, backends, models, currentIDs)
	return s.cleanupStaleBackends(currentIDs)
}

// processBackend routes the backend processing logic based on the backend's Type.
// It acts as a dispatcher to type-specific handling functions (e.g., for Ollama).
// It updates the internal state map with the results of the processing,
// including any errors encountered for unsupported types.
func (s *State) processBackend(ctx context.Context, backend *store.Backend, declaredOllamaModels []*store.Model) {
	switch backend.Type {
	case "Ollama":
		s.processOllamaBackend(ctx, backend, declaredOllamaModels)
	default:
		log.Printf("Unsupported backend type: %s", backend.Type)
		brokenService := &LLMState{
			ID:      backend.ID,
			Name:    backend.Name,
			Models:  []string{},
			Backend: *backend,
			Error:   "Unsupported backend type: " + backend.Type,
		}
		s.state.Store(backend.ID, brokenService)
	}
}

// processOllamaBackend handles the state reconciliation for a single Ollama backend.
// It connects to the Ollama API, compares the set of declared models for this backend
// with the models actually present on the Ollama instance, and takes corrective actions:
// - Queues downloads for declared models that are missing.
// - Initiates deletion for models present on the instance but not declared in the config.
// Finally, it updates the internal state map with the latest observed list of pulled models
// and any communication errors encountered.
func (s *State) processOllamaBackend(ctx context.Context, backend *store.Backend, declaredOllamaModels []*store.Model) {
	log.Printf("Processing Ollama backend for ID %s with declared models: %+v", backend.ID, declaredOllamaModels)

	models := []string{}
	for _, model := range declaredOllamaModels {
		models = append(models, model.Model)
	}
	log.Printf("Extracted model names for backend %s: %v", backend.ID, models)

	backendURL, err := url.Parse(backend.BaseURL)
	if err != nil {
		log.Printf("Error parsing URL for backend %s: %v", backend.ID, err)
		stateservice := &LLMState{
			ID:           backend.ID,
			Name:         backend.Name,
			Models:       models,
			PulledModels: nil,
			Backend:      *backend,
			Error:        "Invalid URL: " + err.Error(),
		}
		s.state.Store(backend.ID, stateservice)
		return
	}
	log.Printf("Parsed URL for backend %s: %s", backend.ID, backendURL.String())

	client := api.NewClient(backendURL, http.DefaultClient)
	existingModels, err := client.List(ctx)
	if err != nil {
		log.Printf("Error listing models for backend %s: %v", backend.ID, err)
		stateservice := &LLMState{
			ID:           backend.ID,
			Name:         backend.Name,
			Models:       models,
			PulledModels: nil,
			Backend:      *backend,
			Error:        err.Error(),
		}
		s.state.Store(backend.ID, stateservice)
		return
	}
	log.Printf("Existing models from backend %s: %+v", backend.ID, existingModels.Models)

	declaredModelSet := make(map[string]struct{})
	for _, declaredModel := range declaredOllamaModels {
		declaredModelSet[declaredModel.Model] = struct{}{}
	}
	log.Printf("Declared model set for backend %s: %v", backend.ID, declaredModelSet)

	existingModelSet := make(map[string]struct{})
	for _, existingModel := range existingModels.Models {
		existingModelSet[existingModel.Model] = struct{}{}
	}
	log.Printf("Existing model set for backend %s: %v", backend.ID, existingModelSet)

	// For each declared model missing from the backend, add a download job.
	for declaredModel := range declaredModelSet {
		if _, ok := existingModelSet[declaredModel]; !ok {
			log.Printf("Model %s is declared but missing in backend %s. Adding to download queue.", declaredModel, backend.ID)
			// RATIONALE: Using the backend URL as the Job ID in the queue prevents
			// queueing multiple downloads for the same backend simultaneously,
			// acting as a simple lock at the queue level.
			// Download flow:
			// 1. The sync cycle re-evaluates the full desired vs. actual state
			//    periodically. It will re-detect *all* currently missing models on each run.
			// 2. Therefore, the queue doesn't need to store a "TODO" list of all
			//    pending downloads for a backend. A single job per backend URL acts as
			//    a sufficient signal that *a* download action is required.
			// 3. The specific model placed in this job's payload reflects one missing model
			//    identified during the *most recent* sync cycle run.
			// 4. When this model is downloaded, the *next* sync cycle will identify the
			//    *next* missing model (if any) and trigger the queue again, eventually
			//    leading to all models being downloaded over successive cycles.
			// 5. If the backeend dies while downloading this mechanism will ensure that
			//    the downloadjob will be readded to the queue.
			err := s.dwQueue.add(ctx, *backendURL, declaredModel)
			if err != nil {
				log.Printf("Error adding model %s to download queue: %v", declaredModel, err)
			}
		}
	}

	// For each model in the backend that is not declared, trigger deletion.
	// NOTE: We have to delete otherwise we have keep track of not desired model in each backend to
	// ensure some backend-nodes don't just run out of space.
	for existingModel := range existingModelSet {
		if _, ok := declaredModelSet[existingModel]; !ok {
			log.Printf("Model %s exists in backend %s but is not declared. Triggering deletion.", existingModel, backend.ID)
			err := client.Delete(ctx, &api.DeleteRequest{
				Model: existingModel,
			})
			if err != nil {
				log.Printf("Error deleting model %s for backend %s: %v", existingModel, backend.ID, err)
			} else {
				log.Printf("Successfully deleted model %s for backend %s", existingModel, backend.ID)
			}
		}
	}

	modelResp, err := client.List(ctx)
	if err != nil {
		log.Printf("Error listing running models for backend %s after deletion: %v", backend.ID, err)
		stateservice := &LLMState{
			ID:           backend.ID,
			Name:         backend.Name,
			Models:       models,
			PulledModels: nil,
			Backend:      *backend,
			Error:        err.Error(),
		}
		s.state.Store(backend.ID, stateservice)
		return
	}
	log.Printf("Updated model list for backend %s: %+v", backend.ID, modelResp.Models)

	stateservice := &LLMState{
		ID:           backend.ID,
		Name:         backend.Name,
		Models:       models,
		PulledModels: modelResp.Models,
		Backend:      *backend,
	}
	s.state.Store(backend.ID, stateservice)
	log.Printf("Stored updated state for backend %s", backend.ID)
}
