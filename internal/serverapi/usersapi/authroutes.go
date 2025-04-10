// Package usersapi provides HTTP handlers for user authentication and management.
package usersapi

import (
	"net/http"
	"time"

	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/services/userservice"
)

const (
	authCookieName = "auth_token"
)

func AddAuthRoutes(mux *http.ServeMux, userService *userservice.Service) {
	a := &authManager{
		userService: userService,
	}

	mux.HandleFunc("POST /login", a.login)       // Resource Owner Password Credentials Flow use only for M2M & BfF
	mux.HandleFunc("POST /register", a.register) // Resource Owner Password Credentials Flow use only for M2M & BfF
	mux.HandleFunc("POST /token_refresh", a.tokenRefresh)

	mux.HandleFunc("GET /ui/me", a.uiMe)
	mux.HandleFunc("POST /ui/login", a.uiLogin)
	mux.HandleFunc("POST /ui/logout", a.uiLogout)
	mux.HandleFunc("POST /ui/register", a.uiRegister)
	mux.HandleFunc("POST /ui/token_refresh", a.uiTokenRefresh)

}

type authManager struct {
	userService *userservice.Service
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (a *authManager) login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := serverops.Decode[loginRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	result, err := a.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, result)
}

func (a *authManager) register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req userservice.CreateUserRequest
	req, err := serverops.Decode[userservice.CreateUserRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	result, err := a.userService.Register(ctx, req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, result)
}

type tokenRefreshRequest struct {
	Token string `json:"token"`
}

func (a *authManager) tokenRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Decode the request containing the old token.
	req, err := serverops.Decode[tokenRefreshRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	// Attempt to refresh the token.
	newToken, _, _, err := serverops.RefreshPlainToken(ctx, req.Token, nil)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	response := struct {
		Token string `json:"token"`
	}{
		Token: newToken,
	}
	_ = serverops.Encode(w, r, http.StatusOK, response)
}

func (a *authManager) uiRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Decode the registration request
	var req userservice.CreateUserRequest
	req, err := serverops.Decode[userservice.CreateUserRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	result, err := a.userService.Register(ctx, req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    result.Token,
		Path:     "/",
		Expires:  result.ExpiresAt,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   false, // TODO: Set to true if using HTTPS
	}
	http.SetCookie(w, cookie)

	_ = serverops.Encode(w, r, http.StatusCreated, result.User)
}

func (a *authManager) uiMe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := a.userService.GetUserFromContext(ctx)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusOK, result)
}

// uiLogin handles a login request by authenticating the user and setting an HTTP-only cookie with the token.
func (a *authManager) uiLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := serverops.Decode[loginRequest](r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	result, err := a.userService.Login(ctx, req.Email, req.Password)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	// Create and set the auth cookie.
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    result.Token,
		Path:     "/",
		Expires:  result.ExpiresAt,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,  // prevents JS access to the cookie
		Secure:   false, // TODO: Set to true if using HTTPS
	}
	http.SetCookie(w, cookie)

	_ = serverops.Encode(w, r, http.StatusOK, result.User)
}

// uiLogout clears the authentication cookie.
func (a *authManager) uiLogout(w http.ResponseWriter, r *http.Request) {
	// Clear the cookie by setting an expired cookie.
	cookie := &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false, // TODO: Set to true if using HTTPS
	}
	http.SetCookie(w, cookie)

	// Return a simple success message.
	_ = serverops.Encode(w, r, http.StatusOK, map[string]string{
		"message": "logout successful",
	})
}

// uiTokenRefresh reads the existing token from the cookie, refreshes it, and updates the cookie.
func (a *authManager) uiTokenRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Retrieve the token from the cookie.
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	// Refresh the token using the service.
	newToken, _, expiresAt, err := serverops.RefreshPlainToken(ctx, cookie.Value, nil)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.AuthorizeOperation)
		return
	}

	// Update the cookie with the new token.
	newCookie := &http.Cookie{
		Name:     authCookieName,
		Value:    newToken,
		Path:     "/",
		Expires:  expiresAt,
		SameSite: http.SameSiteStrictMode,
		HttpOnly: true,
		Secure:   false, // todo: set to true if using HTTPS
	}
	http.SetCookie(w, newCookie)

	_ = serverops.Encode(w, r, http.StatusOK, map[string]string{
		"message": "token refreshed",
	})
}
