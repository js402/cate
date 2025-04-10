package store

import (
	"context"
	_ "embed"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/js402/CATE/libs/libauth"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

type Status struct {
	Status    string `json:"status"`
	Digest    string `json:"digest,omitempty"`
	Total     int64  `json:"total,omitempty"`
	Completed int64  `json:"completed,omitempty"`
	Model     string `json:"model"`
	BaseURL   string `json:"baseUrl"`
}

type QueueItem struct {
	URL   string `json:"url"`
	Model string `json:"model"`
}

type Backend struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
	Type    string `json:"type"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Model struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Pool struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	PurposeType string `json:"purposeType"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	ID               string `json:"id"`
	FriendlyName     string `json:"friendlyName"`
	Email            string `json:"email"`
	Subject          string `json:"subject"`
	HashedPassword   string `json:"hashedPassword"`
	RecoveryCodeHash string `json:"recoveryCodeHash"`
	Salt             string `json:"salt"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Job struct {
	ID           string    `json:"id"`
	TaskType     string    `json:"taskType"`
	Payload      []byte    `json:"payload"`
	ScheduledFor int       `json:"scheduledFor"`
	ValidUntil   int       `json:"validUntil"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Resource struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type File struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Type      string    `json:"type"`
	Meta      []byte    `json:"meta"`
	IsFolder  bool      `json:"isFolder"`
	BlobsID   string    `json:"blobsId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Blob struct {
	ID        string    `json:"id"`
	Meta      []byte    `json:"meta"`
	Data      []byte    `json:"data"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Permission int

const (
	PermissionNone Permission = iota
	PermissionView
	PermissionEdit
	PermissionManage
)

var permissionNames = map[Permission]string{
	PermissionNone:   "none",
	PermissionView:   "view",
	PermissionEdit:   "edit",
	PermissionManage: "manage",
}

var permissionValues = map[string]Permission{
	"none":   PermissionNone,
	"view":   PermissionView,
	"edit":   PermissionEdit,
	"manage": PermissionManage,
}

func (p Permission) String() string {
	if name, exists := permissionNames[p]; exists {
		return name
	}
	return "unknown"
}

func PermissionFromString(s string) (Permission, error) {
	if perm, exists := permissionValues[strings.ToLower(s)]; exists {
		return perm, nil
	}
	return PermissionNone, errors.New("invalid permission string")
}

type AccessEntry struct {
	ID         string     `json:"id"`
	Identity   string     `json:"identity"`
	Resource   string     `json:"resource"`
	Permission Permission `json:"permission"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type AccessList []*AccessEntry

var _ libauth.Authz = AccessList{}

func (al AccessList) RequireAuthorisation(forResource string, permission int) (bool, error) {
	for _, entry := range al {
		if entry.Resource == forResource && entry.Permission >= Permission(permission) {
			return true, nil
		}
	}
	return false, nil
}

type Store interface {
	CreateBackend(ctx context.Context, backend *Backend) error
	GetBackend(ctx context.Context, id string) (*Backend, error)
	UpdateBackend(ctx context.Context, backend *Backend) error
	DeleteBackend(ctx context.Context, id string) error
	ListBackends(ctx context.Context) ([]*Backend, error)
	GetBackendByName(ctx context.Context, name string) (*Backend, error)

	AppendModel(ctx context.Context, model *Model) error
	DeleteModel(ctx context.Context, modelName string) error
	ListModels(ctx context.Context) ([]*Model, error)

	CreatePool(ctx context.Context, pool *Pool) error
	GetPool(ctx context.Context, id string) (*Pool, error)
	GetPoolByName(ctx context.Context, name string) (*Pool, error)
	UpdatePool(ctx context.Context, pool *Pool) error
	DeletePool(ctx context.Context, id string) error
	ListPools(ctx context.Context) ([]*Pool, error)
	ListPoolsByPurpose(ctx context.Context, purposeType string) ([]*Pool, error)

	AssignBackendToPool(ctx context.Context, poolID string, backendID string) error
	RemoveBackendFromPool(ctx context.Context, poolID string, backendID string) error
	ListBackendsForPool(ctx context.Context, poolID string) ([]*Backend, error)
	ListPoolsForBackend(ctx context.Context, backendID string) ([]*Pool, error)

	AssignModelToPool(ctx context.Context, poolID string, modelID string) error
	RemoveModelFromPool(ctx context.Context, poolID string, modelID string) error
	ListModelsForPool(ctx context.Context, poolID string) ([]*Model, error)
	ListPoolsForModel(ctx context.Context, modelID string) ([]*Pool, error)

	AppendJob(ctx context.Context, job Job) error
	PopAllJobs(ctx context.Context) ([]*Job, error)
	PopJobsForType(ctx context.Context, taskType string) ([]*Job, error)
	PopJobForType(ctx context.Context, taskType string) (*Job, error)
	GetJobsForType(ctx context.Context, taskType string) ([]*Job, error)

	CreateAccessEntry(ctx context.Context, entry *AccessEntry) error
	GetAccessEntryByID(ctx context.Context, id string) (*AccessEntry, error)
	UpdateAccessEntry(ctx context.Context, entry *AccessEntry) error
	DeleteAccessEntry(ctx context.Context, id string) error
	DeleteAccessEntriesByIdentity(ctx context.Context, identity string) error
	DeleteAccessEntriesByResource(ctx context.Context, resource string) error
	ListAccessEntries(ctx context.Context, createdAtCursor time.Time) ([]*AccessEntry, error)
	GetAccessEntriesByIdentity(ctx context.Context, identity string) ([]*AccessEntry, error)

	CreateUser(ctx context.Context, user *User) error
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserBySubject(ctx context.Context, subject string) (*User, error)
	UpdateUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error
	ListUsersBySubjects(ctx context.Context, subject ...string) ([]*User, error)
	ListUsers(ctx context.Context, createdAtCursor time.Time) ([]*User, error)

	CreateFile(ctx context.Context, file *File) error
	GetFileByID(ctx context.Context, id string) (*File, error)
	UpdateFile(ctx context.Context, file *File) error
	UpdateFilePath(ctx context.Context, id string, newPath string) error
	BulkUpdateFilePaths(ctx context.Context, updates map[string]string) error
	DeleteFile(ctx context.Context, id string) error
	ListFilesByPath(ctx context.Context, path string) ([]File, error)
	ListFiles(ctx context.Context) ([]string, error)
	EstimateFileCount(ctx context.Context) (int64, error)
	EnforceMaxFileCount(ctx context.Context, maxCount int64) error

	CreateBlob(ctx context.Context, blob *Blob) error
	GetBlobByID(ctx context.Context, id string) (*Blob, error)
	DeleteBlob(ctx context.Context, id string) error
}

//go:embed schema.sql
var Schema string

type store struct {
	libdb.Exec
}

func New(exec libdb.Exec) Store {
	return &store{exec}
}

// setupStore initializes a test Postgres instance and returns the store.
func SetupStore(t *testing.T) (context.Context, Store) {
	t.Helper()
	ctx := context.TODO()
	connStr, _, cleanup, err := libdb.SetupLocalInstance(ctx, "test", "test", "test")
	require.NoError(t, err)

	dbManager, err := libdb.NewPostgresDBManager(ctx, connStr, Schema)
	require.NoError(t, err)

	// Ensure cleanup of resources when the test completes.
	t.Cleanup(func() {
		err := dbManager.Close()
		require.NoError(t, err)
		cleanup()
	})

	s := New(dbManager.WithoutTransaction())
	return ctx, s
}
