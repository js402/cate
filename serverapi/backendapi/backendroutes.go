package backendapi

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/js402/cate/runtimestate"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
	"github.com/js402/cate/services/backendservice"
	"github.com/ollama/ollama/api"
)

func AddBackendRoutes(mux *http.ServeMux, _ *serverops.Config, backendService *backendservice.Service, stateService *runtimestate.State) {
	b := &backendManager{service: backendService, stateService: stateService}

	mux.HandleFunc("POST /backends", b.create)
	mux.HandleFunc("GET /backends", b.list)
	mux.HandleFunc("GET /backends/{id}", b.get)
	mux.HandleFunc("PUT /backends/{id}", b.update)
	mux.HandleFunc("DELETE /backends/{id}", b.delete)
}

type respBackendList struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
	Type    string `json:"type"`

	Models       []string                `json:"models"`
	PulledModels []api.ListModelResponse `json:"pulledModels"`
	Error        string                  `json:"error,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type backendManager struct {
	service      *backendservice.Service
	stateService *runtimestate.State
}

func (b *backendManager) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	backend, err := serverops.Decode[store.Backend](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}
	backend.ID = uuid.NewString()
	if err := b.service.Create(ctx, &backend); err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, backend)
}

func (b *backendManager) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	backends, err := b.service.List(ctx)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	backendState := b.stateService.Get(ctx)
	resp := []respBackendList{}
	for _, backend := range backends {
		item := respBackendList{
			ID:      backend.ID,
			Name:    backend.Name,
			BaseURL: backend.BaseURL,
			Type:    "Ollama",
		}
		state, ok := backendState[backend.ID]
		if ok {
			item.Models = state.Models
			item.PulledModels = state.PulledModels
			item.Error = state.Error
		}
		resp = append(resp, item)
	}

	_ = serverops.Encode(w, r, http.StatusOK, resp)
}

func (b *backendManager) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	backend, err := b.service.Get(ctx, id)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, backend)
}

func (b *backendManager) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	backend, err := serverops.Decode[store.Backend](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	backend.ID = id
	if err := b.service.Update(ctx, &backend); err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, backend)
}

func (b *backendManager) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	if err := b.service.Delete(ctx, id); err != nil {
		_ = serverops.Error(w, r, err, serverops.DeleteOperation)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
