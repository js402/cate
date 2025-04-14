package backendapi

import (
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
	"github.com/js402/cate/services/downloadservice"
	"github.com/js402/cate/services/modelservice"
)

func AddModelRoutes(mux *http.ServeMux, _ *serverops.Config, modelService *modelservice.Service, dwService *downloadservice.Service) {
	s := &service{service: modelService, dwService: dwService}

	mux.HandleFunc("POST /models", s.append)
	mux.HandleFunc("GET /models", s.list)
	mux.HandleFunc("DELETE /models/{model}", s.delete)
}

type service struct {
	service   *modelservice.Service
	dwService *downloadservice.Service
}

func (s *service) append(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	model, err := serverops.Decode[store.Model](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	model.ID = uuid.NewString()
	if err := s.service.Append(ctx, &model); err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, model)
}

func (s *service) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	models, err := s.service.List(ctx)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, models)
}

func (s *service) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelName := url.PathEscape(r.PathValue("model"))
	if modelName == "" {
		serverops.Error(w, r, serverops.ErrBadPathValue("model name required"), serverops.DeleteOperation)
		return
	}
	if err := s.service.Delete(ctx, modelName); err != nil {
		_ = serverops.Error(w, r, err, serverops.DeleteOperation)
		return
	}
	queue := r.URL.Query().Get("purge")
	if queue == "true" {
		if err := s.dwService.RemoveFromQueue(r.Context(), modelName); err != nil {
			_ = serverops.Error(w, r, err, serverops.DeleteOperation)
			return
		}
		if err := s.dwService.CancelDownloads(r.Context(), modelName); err != nil {
			_ = serverops.Error(w, r, err, serverops.DeleteOperation)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
