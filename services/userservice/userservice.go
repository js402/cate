package userservice

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"dario.cat/mergo"
	"github.com/google/uuid"
	"github.com/js402/cate/libs/libcipher"
	"github.com/js402/cate/libs/libdb"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
)

var ErrUserAlreadyExists = errors.New("user already exists")
var ErrTokenGenerationFailed = errors.New("failed to generate token")

type Service struct {
	dbInstance      libdb.DBManager
	securityEnabled bool
	serverSecret    string
	signingKey      string
}

func New(db libdb.DBManager, config *serverops.Config) *Service {
	var securityEnabledFlag bool
	if config.SecurityEnabled == "true" {
		securityEnabledFlag = true
	}

	return &Service{dbInstance: db,
		securityEnabled: securityEnabledFlag,
		serverSecret:    config.JWTSecret,
		signingKey:      config.SigningKey,
	}
}

func (s *Service) GetUserFromContext(ctx context.Context) (*store.User, error) {
	identity, err := serverops.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	// Retrieve user by ID.
	user, err := s.getUserBySubject(ctx, identity)
	if err != nil {
		return nil, err
	}

	user.HashedPassword = ""
	user.RecoveryCodeHash = ""

	return user, nil
}

// Login authenticates a user given an email and password, and returns a JWT on success.
// It verifies the password, loads permissions, and generates a JWT token.
func (s *Service) Login(ctx context.Context, email, password string) (*Result, error) {
	tx := s.dbInstance.WithoutTransaction()

	// Retrieve user by email.
	user, err := s.getUserByEmail(ctx, tx, email)
	if err != nil {
		return nil, err
	}

	// Decode the stored hashed password.
	decodedHash, err := base64.StdEncoding.DecodeString(user.HashedPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to decode stored password: %w", err)
	}

	// Verify password.
	passed, err := libcipher.CheckHash(s.signingKey, user.Salt, password, decodedHash)
	if err != nil || !passed {
		return nil, errors.New("invalid credentials")
	}

	// Load permissions for the user.
	permissions, err := store.New(tx).GetAccessEntriesByIdentity(ctx, user.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to load permissions: %w", err)
	}

	// Use the serverops helper to generate the JWT.
	token, expiresAt, err := serverops.CreateAuthToken(user.Subject, permissions)
	if err != nil {
		return nil, err
	}
	user.HashedPassword = ""
	return &Result{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

// Result bundles the newly registered user and its token.
type Result struct {
	User      *store.User `json:"user"`
	Token     string      `json:"token"`
	ExpiresAt time.Time   `json:"expires_at"`
}

// Register creates a new user and returns a JWT token for that user.
func (s *Service) Register(ctx context.Context, req CreateUserRequest) (*Result, error) {
	tx := s.dbInstance.WithoutTransaction()
	req.AllowedResources = []CreateUserRequestAllowedResources{
		{Name: serverops.DefaultServerGroup, Permission: store.PermissionNone.String()},
	}
	if serverops.DefaultAdminUser == req.Email {
		req.AllowedResources = []CreateUserRequestAllowedResources{
			{Name: serverops.DefaultServerGroup, Permission: store.PermissionManage.String()},
		}
	}
	userFromStore, err := s.createUser(ctx, tx, req)
	if err != nil && !errors.Is(err, libdb.ErrNotFound) {
		return nil, fmt.Errorf("%w %w", ErrUserAlreadyExists, err)
	}
	if err != nil {
		return nil, err
	}

	permissions, err := store.New(tx).GetAccessEntriesByIdentity(ctx, userFromStore.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to load permissions: %w", err)
	}

	// Use the serverops helper to generate the token.
	token, expiresAt, err := serverops.CreateAuthToken(userFromStore.Subject, permissions)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, ErrTokenGenerationFailed
	}
	userFromStore.HashedPassword = ""
	return &Result{User: userFromStore, Token: token, ExpiresAt: expiresAt}, nil
}

type CreateUserRequest struct {
	Email            string                              `json:"email"`
	FriendlyName     string                              `json:"friendlyName,omitempty"`
	Password         string                              `json:"password"`
	AllowedResources []CreateUserRequestAllowedResources `json:"allowedResources"`
}

type CreateUserRequestAllowedResources struct {
	Name       string `json:"name"`
	Permission string `json:"permission"`
}

func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*store.User, error) {
	tx := s.dbInstance.WithoutTransaction()
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	user, err := s.createUser(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	user.HashedPassword = ""
	return user, nil
}

func (s *Service) createUser(ctx context.Context, tx libdb.Exec, req CreateUserRequest) (*store.User, error) {
	user := &store.User{
		ID:           uuid.NewString(),
		Subject:      uuid.NewString(),
		Email:        req.Email,
		FriendlyName: req.FriendlyName,
	}
	if req.Password != "" {
		hashedPassword, err := libcipher.NewHash(libcipher.GenerateHashArgs{Payload: []byte(req.Password), SigningKey: []byte(s.signingKey)}, sha256.New)
		if err != nil {
			return nil, err
		}
		user.HashedPassword = base64.StdEncoding.EncodeToString(hashedPassword)
	}

	err := store.New(tx).CreateUser(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	user, err = store.New(tx).GetUserByID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get created user by id: %w", err)
	}
	for _, curar := range req.AllowedResources {
		perm, err := store.PermissionFromString(curar.Permission)
		if err != nil {
			return nil, err
		}
		err = store.New(tx).CreateAccessEntry(ctx, &store.AccessEntry{
			ID:         uuid.NewString(),
			Identity:   user.Subject,
			Resource:   curar.Name,
			Permission: perm,
		})
		if err != nil {
			return nil, err
		}
	}

	return user, nil
}

func (s *Service) GetUserByID(ctx context.Context, id string) (*store.User, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}

	return s.getUserByID(ctx, id)
}

func (s *Service) getUserByID(ctx context.Context, id string) (*store.User, error) {
	tx := s.dbInstance.WithoutTransaction()
	user, err := store.New(tx).GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return user, err
}

func (s *Service) getUserByEmail(ctx context.Context, tx libdb.Exec, email string) (*store.User, error) {
	user, err := store.New(tx).GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) GetUserBySubject(ctx context.Context, subject string) (*store.User, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	return s.getUserBySubject(ctx, subject)
}

func (s *Service) getUserBySubject(ctx context.Context, subject string) (*store.User, error) {
	tx := s.dbInstance.WithoutTransaction()
	user, err := store.New(tx).GetUserBySubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	return user, nil
}

type UpdateUserRequest struct {
	Email        string `json:"email,omitempty"`
	FriendlyName string `json:"friendlyName,omitempty"`
	Password     string `json:"password"`
}

// UpdateUserFields fetches the user, applies allowed updates, and persists the changes.
func (s *Service) UpdateUserFields(ctx context.Context, id string, req UpdateUserRequest) (*store.User, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}

	tx, commit, rTx, err := s.dbInstance.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}
	// Retrieve the existing user
	user, err := s.GetUserByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update allowed fields
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.FriendlyName != "" {
		user.FriendlyName = req.FriendlyName
	}
	if req.Password != "" {
		hashedPassword, err := libcipher.NewHash(libcipher.GenerateHashArgs{
			Payload:    []byte(req.Password),
			SigningKey: []byte(s.signingKey),
		}, sha256.New)
		if err != nil {
			return nil, err
		}
		user.HashedPassword = base64.StdEncoding.EncodeToString(hashedPassword)
	}

	// Persist the updated user. This method already handles merge logic and duplicate checks.
	if err := s.updateUser(ctx, tx, user); err != nil {
		return nil, err
	}

	if err := commit(ctx); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) updateUser(ctx context.Context, tx libdb.Exec, user *store.User) error {
	userDst, err := store.New(tx).GetUserByID(ctx, user.ID)
	if err != nil {
		return err
	}
	user.CreatedAt = userDst.CreatedAt
	user.RecoveryCodeHash = userDst.RecoveryCodeHash
	user.Subject = userDst.Subject
	err = mergo.Merge(userDst, user, mergo.WithOverride)
	if err != nil {
		return err
	}
	err = store.New(tx).UpdateUser(ctx, userDst)
	if err != nil {
		return err
	}
	return nil
}

func (s *Service) DeleteUser(ctx context.Context, id string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx, commit, rTx, err := s.dbInstance.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return err
	}
	err = store.New(tx).DeleteUser(ctx, id)
	if err != nil {
		return err
	}
	err = store.New(tx).DeleteAccessEntriesByIdentity(ctx, id)
	if err != nil {
		return err
	}
	return commit(ctx)
}

func (s *Service) ListUsers(ctx context.Context, cursorCreatedAt time.Time) ([]*store.User, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	return store.New(tx).ListUsers(ctx, cursorCreatedAt)
}

func (s *Service) GetServiceName() string {
	return "userservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}
