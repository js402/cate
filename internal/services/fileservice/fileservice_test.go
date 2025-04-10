package fileservice_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/internal/services/fileservice"
	"github.com/js402/CATE/libs/libdb"
)

func TestFileService(t *testing.T) {
	ctx := context.Background()
	var cleanups []func()
	addCleanup := func(fn func()) {
		cleanups = append(cleanups, fn)
	}
	defer func() {
		for _, fn := range cleanups {
			fn()
		}
	}()

	dbConn, _, dbCleanup, err := libdb.SetupLocalInstance(ctx, uuid.NewString(), "test", "test")
	if err != nil {
		t.Fatalf("failed to setup local database: %v", err)
	}
	addCleanup(dbCleanup)

	dbInstance, err := libdb.NewPostgresDBManager(ctx, dbConn, store.Schema)
	if err != nil {
		t.Fatalf("failed to create new Postgres DB Manager: %v", err)
	}
	err = serverops.NewServiceManager(&serverops.Config{
		JWTExpiry:       "1h",
		SecurityEnabled: "false",
	})
	if err != nil {
		t.Fatalf("failed to create new Service Manager: %v", err)
	}
	fileService := fileservice.New(dbInstance, &serverops.Config{
		JWTExpiry:       "1h",
		SecurityEnabled: "false",
	})
	err = store.New(dbInstance.WithoutTransaction()).CreateUser(ctx, &store.User{
		Email:        serverops.DefaultAdminUser,
		ID:           uuid.NewString(),
		Subject:      serverops.DefaultAdminUser,
		FriendlyName: "Admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	err = store.New(dbInstance.WithoutTransaction()).CreateAccessEntry(ctx, &store.AccessEntry{
		Identity:   serverops.DefaultAdminUser,
		ID:         uuid.NewString(),
		Resource:   serverops.DefaultServerGroup,
		Permission: store.PermissionManage,
	})
	if err != nil {
		t.Fatalf("failed to create access entry: %v", err)
	}

	t.Run("CreateFile", func(t *testing.T) {
		testFile := &fileservice.File{
			Path:        "test.txt",
			ContentType: "text/plain",
			Data:        []byte("test data"),
		}

		createdFile, err := fileService.CreateFile(ctx, testFile)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		if createdFile.ID == "" {
			t.Error("Expected non-empty ID")
		}
		if createdFile.Path != "test.txt" {
			t.Errorf("Expected path 'test.txt', got %s", createdFile.Path)
		}
		if createdFile.ContentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got %s", createdFile.ContentType)
		}
		if createdFile.Size != int64(len(testFile.Data)) {
			t.Errorf("Expected size %d, got %d", len(testFile.Data), createdFile.Size)
		}

		retrievedFile, err := fileService.GetFileByID(ctx, createdFile.ID)
		if err != nil {
			t.Fatalf("GetFileByID failed: %v", err)
		}
		if !bytes.Equal(retrievedFile.Data, testFile.Data) {
			t.Error("Retrieved file data does not match original")
		}
	})

	t.Run("UpdateFile", func(t *testing.T) {
		originalFile := &fileservice.File{
			Path:        "update.txt",
			ContentType: "text/plain",
			Data:        []byte("original data"),
		}

		createdFile, err := fileService.CreateFile(ctx, originalFile)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		newData := []byte("updated data")
		updateFile := &fileservice.File{
			ID:          createdFile.ID,
			Path:        "updated.txt",
			ContentType: "text/plain",
			Data:        newData,
		}

		updatedFile, err := fileService.UpdateFile(ctx, updateFile)
		if err != nil {
			t.Fatalf("UpdateFile failed: %v", err)
		}

		if updatedFile.Path != "updated.txt" {
			t.Errorf("Expected path 'updated.txt', got %s", updatedFile.Path)
		}
		if !bytes.Equal(updatedFile.Data, newData) {
			t.Error("Data not updated")
		}
		if updatedFile.Size != int64(len(newData)) {
			t.Errorf("Expected size %d, got %d", len(newData), updatedFile.Size)
		}

		retrievedFile, err := fileService.GetFileByID(ctx, createdFile.ID)
		if err != nil {
			t.Fatalf("GetFileByID failed: %v", err)
		}
		if !bytes.Equal(retrievedFile.Data, newData) {
			t.Error("Retrieved data does not match updated data")
		}
	})

	t.Run("DeleteFile", func(t *testing.T) {
		testFile := &fileservice.File{
			Path:        "delete.txt",
			ContentType: "text/plain",
			Data:        []byte("data to delete"),
		}

		createdFile, err := fileService.CreateFile(ctx, testFile)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		err = fileService.DeleteFile(ctx, createdFile.ID)
		if err != nil {
			t.Fatalf("DeleteFile failed: %v", err)
		}

		_, err = fileService.GetFileByID(ctx, createdFile.ID)
		if err == nil {
			t.Error("Expected error when retrieving deleted file, got nil")
		}
	})

	t.Run("CreateFolder", func(t *testing.T) {
		path := "test_folder"
		folder, err := fileService.CreateFolder(ctx, path)
		if err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}

		if folder.ID == "" {
			t.Error("Expected non-empty folder ID")
		}
		if folder.Path != path {
			t.Errorf("Expected path '%s', got '%s'", path, folder.Path)
		}
	})

	t.Run("RenameFile", func(t *testing.T) {
		testFile := &fileservice.File{
			Path:        "oldname.txt",
			ContentType: "text/plain",
			Data:        []byte("data"),
		}

		createdFile, err := fileService.CreateFile(ctx, testFile)
		if err != nil {
			t.Fatalf("CreateFile failed: %v", err)
		}

		newPath := "newname.txt"
		renamedFile, err := fileService.RenameFile(ctx, createdFile.ID, newPath)
		if err != nil {
			t.Fatalf("RenameFile failed: %v", err)
		}

		if renamedFile.Path != newPath {
			t.Errorf("Expected path '%s', got '%s'", newPath, renamedFile.Path)
		}

		retrievedFile, err := fileService.GetFileByID(ctx, createdFile.ID)
		if err != nil {
			t.Fatalf("GetFileByID failed: %v", err)
		}
		if retrievedFile.Path != newPath {
			t.Errorf("Retrieved file path is '%s', expected '%s'", retrievedFile.Path, newPath)
		}
	})

	t.Run("RenameFolder", func(t *testing.T) {
		folderPath := "old_folder"
		folder, err := fileService.CreateFolder(ctx, folderPath)
		if err != nil {
			t.Fatalf("CreateFolder failed: %v", err)
		}

		file1 := &fileservice.File{
			Path:        folderPath + "/file1.txt",
			ContentType: "text/plain",
			Data:        []byte("data1"),
		}
		createdFile1, err := fileService.CreateFile(ctx, file1)
		if err != nil {
			t.Fatalf("CreateFile failed for file1: %v", err)
		}

		file2 := &fileservice.File{
			Path:        folderPath + "/sub/file2.txt",
			ContentType: "text/plain",
			Data:        []byte("data2"),
		}
		createdFile2, err := fileService.CreateFile(ctx, file2)
		if err != nil {
			t.Fatalf("CreateFile failed for file2: %v", err)
		}

		newFolderPath := "new_folder"
		renamedFolder, err := fileService.RenameFolder(ctx, folder.ID, newFolderPath)
		if err != nil {
			t.Fatalf("RenameFolder failed: %v", err)
		}

		if renamedFolder.Path != newFolderPath {
			t.Errorf("Folder path expected '%s', got '%s'", newFolderPath, renamedFolder.Path)
		}

		retrievedFile1, err := fileService.GetFileByID(ctx, createdFile1.ID)
		if err != nil {
			t.Fatalf("GetFileByID failed for file1: %v", err)
		}
		expectedPath1 := newFolderPath + "/file1.txt"
		if retrievedFile1.Path != expectedPath1 {
			t.Errorf("File1 path expected '%s', got '%s'", expectedPath1, retrievedFile1.Path)
		}

		retrievedFile2, err := fileService.GetFileByID(ctx, createdFile2.ID)
		if err != nil {
			t.Fatalf("GetFileByID failed for file2: %v", err)
		}
		expectedPath2 := newFolderPath + "/sub/file2.txt"
		if retrievedFile2.Path != expectedPath2 {
			t.Errorf("File2 path expected '%s', got '%s'", expectedPath2, retrievedFile2.Path)
		}
	})

	t.Run("ListAllPaths", func(t *testing.T) {
		paths := []string{
			"path1.txt",
			"path2.txt",
			"folder1",
			"folder1/file1.txt",
		}

		for _, path := range paths {
			if strings.Contains(path, ".") {
				file := &fileservice.File{
					Path:        path,
					ContentType: "text/plain",
					Data:        []byte("data"),
				}
				_, err := fileService.CreateFile(ctx, file)
				if err != nil {
					t.Fatalf("CreateFile failed for %s: %v", path, err)
				}
			} else {
				_, err := fileService.CreateFolder(ctx, path)
				if err != nil {
					t.Fatalf("CreateFolder failed for %s: %v", path, err)
				}
			}
		}

		listedPaths, err := fileService.ListAllPaths(ctx)
		if err != nil {
			t.Fatalf("ListAllPaths failed: %v", err)
		}

		expectedPaths := make(map[string]bool)
		for _, p := range paths {
			expectedPaths[p] = true
		}

		for _, listed := range listedPaths {
			delete(expectedPaths, listed)
		}

		for p := range expectedPaths {
			t.Errorf("Path not listed: %s", p)
		}
	})
	t.Run("RenameFile_ConflictWithExistingFile", func(t *testing.T) {
		existingFile := &fileservice.File{
			Path:        "conflict.txt",
			ContentType: "text/plain",
			Data:        []byte("existing"),
		}
		_, err := fileService.CreateFile(ctx, existingFile)
		if err != nil {
			t.Fatalf("CreateFile failed for existing file: %v", err)
		}

		fileToRename := &fileservice.File{
			Path:        "original.txt",
			ContentType: "text/plain",
			Data:        []byte("to be renamed"),
		}
		createdFile, err := fileService.CreateFile(ctx, fileToRename)
		if err != nil {
			t.Fatalf("CreateFile failed for file to rename: %v", err)
		}

		_, err = fileService.RenameFile(ctx, createdFile.ID, "conflict.txt")
		if err == nil {
			t.Error("Expected error when renaming to existing file path, got nil")
		}
	})
	t.Run("RenameFolder_ConflictWithExistingFolder", func(t *testing.T) {
		// Create destination folder
		_, err := fileService.CreateFolder(ctx, "existing_folder")
		if err != nil {
			t.Fatalf("CreateFolder failed for existing_folder: %v", err)
		}

		// Create folder to rename
		folderToRename, err := fileService.CreateFolder(ctx, "folder_to_rename")
		if err != nil {
			t.Fatalf("CreateFolder failed for folder_to_rename: %v", err)
		}

		_, err = fileService.RenameFolder(ctx, folderToRename.ID, "existing_folder")
		if err == nil {
			t.Error("Expected error when renaming to existing folder path, got nil")
		}
	})
}
