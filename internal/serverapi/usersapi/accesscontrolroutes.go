package usersapi

import (
	"net/http"
	"time"

	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/internal/services/accessservice"
)

func AddAccessRoutes(mux *http.ServeMux, _ *serverops.Config, accessService *accessservice.Service) {
	a := &accessManager{service: accessService}

	mux.HandleFunc("POST /access-control", a.create)
	mux.HandleFunc("GET /access-control", a.list)
	mux.HandleFunc("GET /permissions", a.permissions)
	mux.HandleFunc("GET /access-control/{id}", a.getByID)
	mux.HandleFunc("PUT /access-control/{id}", a.update)
	mux.HandleFunc("DELETE /access-control/{id}", a.delete)
}

type accessManager struct {
	service *accessservice.Service
}

func (a *accessManager) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	entry, err := serverops.Decode[accessservice.AccessEntryRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	resp, err := a.service.Create(ctx, &entry)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, resp)
}

func (a *accessManager) list(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	from := r.URL.Query().Get("from")
	if from == "" {
		from = time.Now().Format(time.RFC3339)
	}
	starting, err := time.Parse(time.RFC3339, from)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	identity := r.URL.Query().Get("identity")
	withUserDetails := false
	expand := r.URL.Query().Get("expand")
	if expand == "user" {
		withUserDetails = true
	}
	var entries []accessservice.AccessEntryRequest

	if identity != "" {
		entries, err = a.service.ListByIdentity(ctx, identity, withUserDetails)
	} else {
		entries, err = a.service.ListAll(ctx, starting, withUserDetails)
	}
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, entries)
}

func (a *accessManager) permissions(w http.ResponseWriter, r *http.Request) {
	permissions := []string{
		store.PermissionNone.String(),
		store.PermissionView.String(),
		store.PermissionEdit.String(),
		store.PermissionManage.String(),
	}
	_ = serverops.Encode(w, r, http.StatusOK, permissions)
}

func (a *accessManager) getByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	withUserDetails := false
	expand := r.URL.Query().Get("expand")
	if expand == "user" {
		withUserDetails = true
	}
	entry, err := a.service.GetByID(ctx, accessservice.AccessEntryRequest{
		ID:              id,
		WithUserDetails: &withUserDetails,
	})
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, entry)
}

func (a *accessManager) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	entry, err := serverops.Decode[accessservice.AccessEntryRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}
	entry.ID = id

	resp, err := a.service.Update(ctx, &entry)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, resp)
}

func (a *accessManager) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		_ = serverops.Error(w, r, serverops.ErrBadPathValue("missing required parameters"), serverops.AuthorizeOperation)
		return
	}
	if err := a.service.Delete(ctx, id); err != nil {
		_ = serverops.Error(w, r, err, serverops.DeleteOperation)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
