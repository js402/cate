package usersapi

import (
	"net/http"
	"time"

	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
	"github.com/js402/cate/services/userservice"
)

func AddUserRoutes(mux *http.ServeMux, config *serverops.Config, userService *userservice.Service) {
	u := &userManager{service: userService}

	mux.HandleFunc("POST /users", u.create)
	mux.HandleFunc("GET /users", u.list)
	mux.HandleFunc("GET /users/{id}", u.get)
	mux.HandleFunc("PUT /users/{id}", u.update)
	mux.HandleFunc("DELETE /users/{id}", u.delete)
}

type userManager struct {
	service *userservice.Service
}

type userResponse struct {
	ID           string    `json:"id"`
	FriendlyName string    `json:"friendlyName,omitempty"`
	Email        string    `json:"email"`
	Subject      string    `json:"subject"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (u *userManager) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req userservice.CreateUserRequest
	var err error
	if req, err = serverops.Decode[userservice.CreateUserRequest](r); err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}
	user, err := u.service.CreateUser(ctx, req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, mapUserToResponse(user))
}

func (u *userManager) get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	user, err := u.service.GetUserByID(ctx, id)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, mapUserToResponse(user))
}

func (u *userManager) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	req, err := serverops.Decode[userservice.UpdateUserRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	updatedUser, err := u.service.UpdateUserFields(ctx, id, req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, mapUserToResponse(updatedUser))
}

func (u *userManager) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if err := u.service.DeleteUser(ctx, id); err != nil {
		_ = serverops.Error(w, r, err, serverops.DeleteOperation)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (u *userManager) list(w http.ResponseWriter, r *http.Request) {
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
	users, err := u.service.ListUsers(ctx, starting)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}

	response := make([]userResponse, 0, len(users))
	for _, user := range users {
		response = append(response, mapUserToResponse(user))
	}

	_ = serverops.Encode(w, r, http.StatusOK, response)
}

func mapUserToResponse(u *store.User) userResponse {
	return userResponse{
		ID:           u.ID,
		FriendlyName: u.FriendlyName,
		Email:        u.Email,
		Subject:      u.Subject,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}
}
