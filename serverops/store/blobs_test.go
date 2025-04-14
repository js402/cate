package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/cate/libs/libdb"
	"github.com/js402/cate/serverops/store"
	"github.com/stretchr/testify/require"
)

// TestCreateAndGetBlob verifies that a blob can be created and then retrieved by its ID.
func TestCreateAndGetBlob(t *testing.T) {
	ctx, s := store.SetupStore(t)

	blob := &store.Blob{
		ID:   uuid.NewString(),
		Meta: []byte(`{"description": "Test blob"}`),
		Data: []byte("This is some binary data"),
	}

	// Create the blob
	err := s.CreateBlob(ctx, blob)
	require.NoError(t, err)
	require.NotZero(t, blob.CreatedAt)
	require.NotZero(t, blob.UpdatedAt)

	// Retrieve the blob by ID
	retrieved, err := s.GetBlobByID(ctx, blob.ID)
	require.NoError(t, err)
	require.Equal(t, blob.ID, retrieved.ID)
	require.Equal(t, blob.Meta, retrieved.Meta)
	require.Equal(t, blob.Data, retrieved.Data)
	require.WithinDuration(t, blob.CreatedAt, retrieved.CreatedAt, time.Second)
	require.WithinDuration(t, blob.UpdatedAt, retrieved.UpdatedAt, time.Second)
}

// TestGetBlobByIDNotFound verifies that attempting to retrieve a non-existent blob returns ErrNotFound.
func TestGetBlobByIDNotFound(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Attempt to get a blob with a random ID that hasn't been created.
	_, err := s.GetBlobByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

// TestDeleteBlob verifies that a blob can be deleted successfully.
func TestDeleteBlob(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Create a blob to delete.
	blob := &store.Blob{
		ID:   uuid.NewString(),
		Meta: []byte(`{"description": "To be deleted"}`),
		Data: []byte("Some data to be deleted"),
	}
	require.NoError(t, s.CreateBlob(ctx, blob))

	// Delete the blob.
	require.NoError(t, s.DeleteBlob(ctx, blob.ID))

	// Ensure that retrieving the deleted blob returns ErrNotFound.
	_, err := s.GetBlobByID(ctx, blob.ID)
	require.ErrorIs(t, err, libdb.ErrNotFound)
}
