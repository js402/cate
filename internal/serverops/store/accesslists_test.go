package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGetAccessEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create new entry
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: store.PermissionManage,
	}
	err := s.CreateAccessEntry(ctx, entry)
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)
	require.NotEmpty(t, entry.CreatedAt)
	require.NotEmpty(t, entry.UpdatedAt)

	// Retrieve by ID
	fetched, err := s.GetAccessEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, entry.ID, fetched.ID)
	require.Equal(t, "user|123", fetched.Identity)
	require.Equal(t, "project:456", fetched.Resource)
	require.Equal(t, store.PermissionManage, fetched.Permission)
	require.WithinDuration(t, entry.CreatedAt, fetched.CreatedAt, time.Second)
	require.WithinDuration(t, entry.UpdatedAt, fetched.UpdatedAt, time.Second)
}

func TestUpdateAccessEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create initial entry
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: store.PermissionEdit,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))

	// Update entry
	entry.Permission = store.PermissionManage
	entry.Resource = "project:789"
	require.NoError(t, s.UpdateAccessEntry(ctx, entry))

	// Verify changes
	updated, err := s.GetAccessEntryByID(ctx, entry.ID)
	require.NoError(t, err)
	require.Equal(t, store.PermissionManage, updated.Permission)
	require.Equal(t, "project:789", updated.Resource)
	require.True(t, updated.UpdatedAt.After(entry.CreatedAt))
}

func TestDeleteAccessEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create entry
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))

	// Delete entry
	err := s.DeleteAccessEntry(ctx, entry.ID)
	require.NoError(t, err)

	// Verify deletion
	_, err = s.GetAccessEntryByID(ctx, entry.ID)
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

func TestDeleteAccessEntriesByIdentity(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create entry
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))
	// Create entry
	entry = &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:457",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))
	ae, err := s.GetAccessEntriesByIdentity(ctx, "user|123")
	require.Len(t, ae, 2)
	// Delete entry
	err = s.DeleteAccessEntriesByIdentity(ctx, "user|123")
	require.NoError(t, err)

	// Verify deletion
	ae, err = s.GetAccessEntriesByIdentity(ctx, "user|123")
	require.Len(t, ae, 0)
	require.NoError(t, err)
}

func TestDeleteAccessEntriesByResource(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create entry
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))
	// Create entry
	entry = &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:457",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))
	// Delete entry
	err := s.DeleteAccessEntriesByResource(ctx, "project:456")
	require.NoError(t, err)

	// Verify deletion
	ae, err := s.GetAccessEntriesByIdentity(ctx, "user|123")
	require.Len(t, ae, 1)
	require.NoError(t, err)
}

func TestGetAllAccessEntriesOrder(t *testing.T) {
	ctx, s := store.SetupStore(t)
	beforeCreated := time.Now().UTC()
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|1",
		FriendlyName: "Test User",
	}
	require.NoError(t, s.CreateUser(ctx, user))
	user = &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example2.com",
		Subject:      "user|2",
		FriendlyName: "Test User",
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create two entries with delay
	entry1 := &store.AccessEntry{ID: uuid.NewString(), Identity: "user|1", Resource: "res1", Permission: 1}
	require.NoError(t, s.CreateAccessEntry(ctx, entry1))

	time.Sleep(10 * time.Millisecond)

	entry2 := &store.AccessEntry{ID: uuid.NewString(), Identity: "user|2", Resource: "res2", Permission: 2}
	require.NoError(t, s.CreateAccessEntry(ctx, entry2))

	// Verify order (newest first)
	entries, err := s.ListAccessEntries(ctx, time.Now().UTC())
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, entry2.ID, entries[0].ID)
	require.Equal(t, entry1.ID, entries[1].ID)
	entries, err = s.ListAccessEntries(ctx, beforeCreated)
	require.NoError(t, err)
	require.Len(t, entries, 0)
}

func TestGetAccessEntriesByIdentity(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))
	user = &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example2.com",
		Subject:      "user|456",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))

	// Create test entries
	entries := []*store.AccessEntry{
		{ID: uuid.NewString(), Identity: "user|123", Resource: "res1", Permission: 1},
		{ID: uuid.NewString(), Identity: "user|123", Resource: "res2", Permission: 2},
		{ID: uuid.NewString(), Identity: "user|456", Resource: "res1", Permission: 2},
	}

	for _, e := range entries {
		require.NoError(t, s.CreateAccessEntry(ctx, e))
	}

	// Get by identity
	results, err := s.GetAccessEntriesByIdentity(ctx, "user|123")
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func TestUpdateNonExistentEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)

	entry := &store.AccessEntry{
		ID: uuid.NewString(),
	}
	err := s.UpdateAccessEntry(ctx, entry)
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

func TestDeleteNonExistentEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)

	err := s.DeleteAccessEntry(ctx, uuid.NewString())
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

func TestCreateDuplicateEntry(t *testing.T) {
	ctx, s := store.SetupStore(t)
	user := &store.User{
		ID:           uuid.NewString(),
		Email:        "user@example.com",
		Subject:      "user|123",
		FriendlyName: "Test User",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, s.CreateUser(ctx, user))
	entry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   "user|123",
		Resource:   "project:456",
		Permission: 1,
	}
	require.NoError(t, s.CreateAccessEntry(ctx, entry))

	// Attempt duplicate
	err := s.CreateAccessEntry(ctx, entry)
	require.Error(t, err)
	require.ErrorIs(t, err, libdb.ErrUniqueViolation)
}
