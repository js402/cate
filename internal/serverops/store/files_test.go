package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

// TestCreateAndGetFile verifies that a file can be created and retrieved by its ID.
func TestCreateAndGetFile(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Create a new file
	file := &store.File{
		ID:      uuid.NewString(),
		Path:    "/path/to/file.txt",
		Type:    "text/plain",
		Meta:    []byte(`{"description": "Test file"}`),
		BlobsID: uuid.NewString(),
	}

	err := s.CreateFile(ctx, file)
	require.NoError(t, err)
	require.NotZero(t, file.CreatedAt)
	require.NotZero(t, file.UpdatedAt)

	// Retrieve the file by ID
	retrieved, err := s.GetFileByID(ctx, file.ID)
	require.NoError(t, err)
	require.Equal(t, file.ID, retrieved.ID)
	require.Equal(t, file.Path, retrieved.Path)
	require.Equal(t, file.Type, retrieved.Type)
	require.Equal(t, file.Meta, retrieved.Meta)
	require.Equal(t, file.BlobsID, retrieved.BlobsID)
	require.WithinDuration(t, file.CreatedAt, retrieved.CreatedAt, time.Second)
	require.WithinDuration(t, file.UpdatedAt, retrieved.UpdatedAt, time.Second)
}

// TestGetFilesByPath verifies that files can be retrieved by path.
func TestGetFilesByPath(t *testing.T) {
	ctx, s := store.SetupStore(t)

	path := "/common/path/"
	files, err := s.ListFilesByPath(ctx, path)
	require.NoError(t, err)
	require.Len(t, files, 0)
	// Create several files with the same path
	file1 := &store.File{
		ID:      uuid.NewString(),
		Path:    path + uuid.NewString(),
		Type:    "text/plain",
		Meta:    []byte(`{"description": "File 1"}`),
		BlobsID: uuid.NewString(),
	}
	file2 := &store.File{
		ID:      uuid.NewString(),
		Path:    path + uuid.NewString(),
		Type:    "text/plain",
		Meta:    []byte(`{"description": "File 2"}`),
		BlobsID: uuid.NewString(),
	}

	require.NoError(t, s.CreateFile(ctx, file1))
	require.NoError(t, s.CreateFile(ctx, file2))

	files, err = s.ListFilesByPath(ctx, path)
	require.NoError(t, err)
	require.Len(t, files, 2)

	// Optionally verify that the returned files match the ones inserted.
	ids := map[string]bool{file1.ID: true, file2.ID: true}
	for _, f := range files {
		require.True(t, ids[f.ID])
	}
}

// TestUpdateFile verifies that a file's fields can be updated.
func TestUpdateFile(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Create a file to update.
	file := &store.File{
		ID:      uuid.NewString(),
		Path:    "/old/path/file.txt",
		Type:    "text/plain",
		Meta:    []byte(`{"description": "Old description"}`),
		BlobsID: uuid.NewString(),
	}
	require.NoError(t, s.CreateFile(ctx, file))

	// Update file fields.
	file.Path = "/new/path/file.txt"
	file.Type = "application/json"
	file.Meta = []byte(`{"description": "New description"}`)
	file.BlobsID = uuid.NewString()

	// Call update.
	require.NoError(t, s.UpdateFile(ctx, file))

	// Retrieve the file and verify the changes.
	updated, err := s.GetFileByID(ctx, file.ID)
	require.NoError(t, err)
	require.Equal(t, "/new/path/file.txt", updated.Path)
	require.Equal(t, "application/json", updated.Type)
	require.Equal(t, file.Meta, updated.Meta)
	require.Equal(t, file.BlobsID, updated.BlobsID)
	require.True(t, updated.UpdatedAt.After(updated.CreatedAt))
}

// TestDeleteFile verifies that a file can be deleted.
func TestDeleteFile(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Create a file to delete.
	file := &store.File{
		ID:      uuid.NewString(),
		Path:    "/path/to/delete.txt",
		Type:    "text/plain",
		Meta:    []byte(`{"description": "To be deleted"}`),
		BlobsID: uuid.NewString(),
	}
	require.NoError(t, s.CreateFile(ctx, file))

	// Delete the file.
	require.NoError(t, s.DeleteFile(ctx, file.ID))

	// Attempt to retrieve the deleted file.
	_, err := s.GetFileByID(ctx, file.ID)
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

// TestGetFileByIDNotFound verifies that retrieving a non-existent file returns an appropriate error.
func TestGetFileByIDNotFound(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Attempt to get a file that doesn't exist.
	_, err := s.GetFileByID(ctx, uuid.NewString())
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

func TestListAll(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Initially, there should be no file files.
	files, err := s.ListFiles(ctx)
	require.NoError(t, err)
	require.Len(t, files, 0)

	// Insert several files with various paths.
	file1 := &store.File{
		ID:      uuid.NewString(),
		Path:    "/path/one/1",
		Type:    "text/plain",
		Meta:    []byte(`{"description": "File one"}`),
		BlobsID: uuid.NewString(),
	}
	file2 := &store.File{
		ID:      uuid.NewString(),
		Path:    "/path/two",
		Type:    "text/plain",
		Meta:    []byte(`{"description": "File two"}`),
		BlobsID: uuid.NewString(),
	}
	// Duplicate path with file1.
	file3 := &store.File{
		ID:      uuid.NewString(),
		Path:    "/path/one",
		Type:    "application/json",
		Meta:    []byte(`{"description": "Another file at path one"}`),
		BlobsID: uuid.NewString(),
	}

	require.NoError(t, s.CreateFile(ctx, file1))
	require.NoError(t, s.CreateFile(ctx, file2))
	require.NoError(t, s.CreateFile(ctx, file3))

	// List all.
	files, err = s.ListFiles(ctx)
	require.NoError(t, err)
	// Expecting only two distinct paths: "/path/one" and "/path/two".
	require.Len(t, files, 3)

	// Optionally verify that the returned paths are correct.
	distinctPaths := map[string]bool{}
	for _, p := range files {
		distinctPaths[p] = true
	}
	require.True(t, distinctPaths["/path/one"], "Expected path '/path/one' to be present")
	require.True(t, distinctPaths["/path/two"], "Expected path '/path/two' to be present")
}
func TestBulkUpdateFilePaths_Success(t *testing.T) {
	ctx, s := store.SetupStore(t)

	// Create test files
	file1 := &store.File{
		ID:      uuid.NewString(),
		Path:    "/bulk/old1",
		Type:    "text/plain",
		Meta:    []byte(`{"desc": "1"}`),
		BlobsID: uuid.NewString(),
	}
	file2 := &store.File{
		ID:      uuid.NewString(),
		Path:    "/bulk/old2",
		Type:    "text/plain",
		Meta:    []byte(`{"desc": "2"}`),
		BlobsID: uuid.NewString(),
	}
	require.NoError(t, s.CreateFile(ctx, file1))
	require.NoError(t, s.CreateFile(ctx, file2))

	originalUpdatedAt1 := file1.UpdatedAt
	originalUpdatedAt2 := file2.UpdatedAt

	// Prepare update
	updates := map[string]string{
		file1.ID: "/bulk/new1",
		file2.ID: "/bulk/new2",
	}

	time.Sleep(1 * time.Second) // ensure UpdatedAt changes
	require.NoError(t, s.BulkUpdateFilePaths(ctx, updates))

	// Verify updates
	updated1, err := s.GetFileByID(ctx, file1.ID)
	require.NoError(t, err)
	require.Equal(t, "/bulk/new1", updated1.Path)
	require.True(t, updated1.UpdatedAt.After(originalUpdatedAt1))

	updated2, err := s.GetFileByID(ctx, file2.ID)
	require.NoError(t, err)
	require.Equal(t, "/bulk/new2", updated2.Path)
	require.True(t, updated2.UpdatedAt.After(originalUpdatedAt2))
}
