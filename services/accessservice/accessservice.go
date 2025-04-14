package accessservice

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/js402/cate/libs/libdb"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
)

type Service struct {
	dbInstance      libdb.DBManager
	securityEnabled bool
	jwtSecret       string
}

func New(db libdb.DBManager) *Service {
	return &Service{dbInstance: db}
}

type AccessEntryRequest struct {
	ID         string `json:"id"`
	Identity   string `json:"identity"`
	Resource   string `json:"resource"`
	Permission string `json:"permission"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	WithUserDetails *bool         `json:"withUserDetails,omitempty"`
	IdentityDetails *UserMetadata `json:"identityDetails,omitempty"`
}

type UserMetadata struct {
	ID           string `json:"id"`
	FriendlyName string `json:"friendlyName"`
	Email        string `json:"email"`
	Subject      string `json:"subject"`
}

func (s *Service) Create(ctx context.Context, entry *AccessEntryRequest) (*AccessEntryRequest, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	perm, err := store.PermissionFromString(entry.Permission)
	if err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	id := uuid.NewString()
	err = store.New(tx).CreateAccessEntry(ctx, &store.AccessEntry{
		ID:         id,
		Identity:   entry.Identity,
		Permission: perm,
		Resource:   entry.Resource,
	})
	if err != nil {
		return nil, err
	}
	withDetails := false
	if entry.WithUserDetails != nil && *entry.WithUserDetails {
		withDetails = true
	}
	return s.getByID(ctx, tx, id, withDetails)
}

func (s *Service) GetByID(ctx context.Context, entry AccessEntryRequest) (*AccessEntryRequest, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	withDetails := false
	if entry.WithUserDetails != nil && *entry.WithUserDetails {
		withDetails = true
	}
	return s.getByID(ctx, tx, entry.ID, withDetails)
}

func (s *Service) getByID(ctx context.Context, tx libdb.Exec, id string, withUser bool) (*AccessEntryRequest, error) {
	entry, err := store.New(tx).GetAccessEntryByID(ctx, id)
	if err != nil {
		return nil, err
	}
	res := &AccessEntryRequest{
		Identity:   entry.Identity,
		Resource:   entry.Resource,
		Permission: entry.Permission.String(),
		UpdatedAt:  entry.UpdatedAt,
		CreatedAt:  entry.CreatedAt,
	}
	if withUser {
		user, err := store.New(tx).GetUserBySubject(ctx, entry.Identity)
		if err != nil {
			return nil, err
		}
		res.IdentityDetails = &UserMetadata{
			ID:           user.ID,
			Subject:      user.Subject,
			Email:        user.Email,
			FriendlyName: user.FriendlyName,
		}
	}
	return res, err
}

func (s *Service) Update(ctx context.Context, entry *AccessEntryRequest) (*AccessEntryRequest, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	perm, err := store.PermissionFromString(entry.Permission)
	if err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	err = store.New(tx).UpdateAccessEntry(ctx, &store.AccessEntry{
		ID:         entry.ID,
		Identity:   entry.Identity,
		Permission: perm,
		Resource:   entry.Resource,
	})
	withDetails := false
	if entry.WithUserDetails != nil && *entry.WithUserDetails {
		withDetails = true
	}
	return s.getByID(ctx, tx, entry.ID, withDetails)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx := s.dbInstance.WithoutTransaction()
	err := store.New(tx).DeleteAccessEntry(ctx, id)
	return err
}

func (s *Service) ListAll(ctx context.Context, starting time.Time, withDetails bool) ([]AccessEntryRequest, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	entries, err := store.New(tx).ListAccessEntries(ctx, starting)
	if err != nil {
		return nil, err
	}
	cE := make([]AccessEntryRequest, len(entries))
	subjects := make([]string, len(entries))
	for i := range entries {
		cE[i] = AccessEntryRequest{
			ID:         entries[i].ID,
			Identity:   entries[i].Identity,
			Permission: entries[i].Permission.String(),
			Resource:   entries[i].Resource,
		}
		subjects[i] = entries[i].Identity
	}
	if withDetails {
		users, err := store.New(tx).ListUsersBySubjects(ctx, subjects...)
		if err != nil {
			return nil, err
		}
		for i, u := range users {
			cE[i].IdentityDetails = &UserMetadata{
				ID:           u.ID,
				Subject:      u.Subject,
				Email:        u.Email,
				FriendlyName: u.FriendlyName,
			}
		}
	}
	return cE, err
}

func (s *Service) ListByIdentity(ctx context.Context, identity string, withDetails bool) ([]AccessEntryRequest, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}
	tx := s.dbInstance.WithoutTransaction()
	entries, err := store.New(tx).GetAccessEntriesByIdentity(ctx, identity)
	if err != nil {
		return nil, err
	}
	cE := make([]AccessEntryRequest, len(entries))
	subjects := make([]string, len(entries))
	for i := range entries {
		cE[i] = AccessEntryRequest{
			ID:         entries[i].ID,
			Identity:   entries[i].Identity,
			Permission: entries[i].Permission.String(),
			Resource:   entries[i].Resource,
		}
		subjects[i] = entries[i].Identity
	}
	if withDetails {
		users, err := store.New(tx).ListUsersBySubjects(ctx, subjects...)
		if err != nil {
			return nil, err
		}
		for i, u := range users {
			cE[i].IdentityDetails = &UserMetadata{
				ID:           u.ID,
				Subject:      u.Subject,
				Email:        u.Email,
				FriendlyName: u.FriendlyName,
			}
		}
	}
	return cE, err
}

type LoginArgs struct {
	Subject    string
	SigningKey []byte
	Password   string
	JWTSecret  string
	JWTExpiry  string
}

func (s *Service) GetServiceName() string {
	return "accessservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}
