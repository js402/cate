package fileservice_test

import (
	"context"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/internal/services/fileservice"
	"github.com/js402/CATE/libs/libdb"
)

const benchmarkFileSize = 1024 * 1024 // 1MB for data-intensive benchmarks

// setupFileServiceBenchmark sets up the database, service manager, and fileService.
// It returns the fileService and a cleanup function.
func setupFileServiceBenchmark(ctx context.Context, t testing.TB) (*fileservice.Service, func()) {
	t.Helper()
	var cleanups []func()
	addCleanup := func(fn func()) {
		cleanups = append(cleanups, fn)
	}

	dbConn, _, dbCleanup, err := libdb.SetupLocalInstance(ctx, uuid.NewString(), "test", "test")
	if err != nil {
		t.Fatalf("failed to setup local database: %v", err)
	}
	addCleanup(dbCleanup)

	dbInstance, err := libdb.NewPostgresDBManager(ctx, dbConn, store.Schema)
	if err != nil {
		t.Fatalf("failed to create new Postgres DB Manager: %v", err)
	}

	// Initialize the global Service Manager.
	err = serverops.NewServiceManager(&serverops.Config{
		JWTExpiry:       "1h",
		SecurityEnabled: "false",
	})
	if err != nil {
		t.Fatalf("failed to create new Service Manager: %v", err)
	}

	// Create the file service.
	fileService := fileservice.New(dbInstance, &serverops.Config{
		JWTExpiry:       "1h",
		SecurityEnabled: "false",
	})

	// Create default admin user.
	err = store.New(dbInstance.WithoutTransaction()).CreateUser(ctx, &store.User{
		Email:        serverops.DefaultAdminUser,
		ID:           uuid.NewString(),
		Subject:      serverops.DefaultAdminUser,
		FriendlyName: "Admin",
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create an access entry for the default admin.
	err = store.New(dbInstance.WithoutTransaction()).CreateAccessEntry(ctx, &store.AccessEntry{
		Identity:   serverops.DefaultAdminUser,
		ID:         uuid.NewString(),
		Resource:   serverops.DefaultServerGroup,
		Permission: store.PermissionManage,
	})
	if err != nil {
		t.Fatalf("failed to create access entry: %v", err)
	}

	return fileService, func() {
		for _, fn := range cleanups {
			fn()
		}
	}
}

func createFileForBenchmark(ctx context.Context, b *testing.B, fs *fileservice.Service, path string, data []byte, contentType string) *fileservice.File {
	b.Helper()
	file := &fileservice.File{
		Path:        path,
		ContentType: contentType,
		Data:        data,
	}
	created, err := fs.CreateFile(ctx, file)
	if err != nil {
		b.Fatalf("CreateFile failed: %v", err)
	}
	return created
}

func showOpsPerSecond(b *testing.B, ops int64) {
	b.Helper()
	elapsed := b.Elapsed().Seconds()
	if elapsed > 0 {
		opsPerSec := float64(ops) / elapsed
		b.ReportMetric(opsPerSec, "ops/s")
	}
}

func createFolderForBenchmark(ctx context.Context, b *testing.B, fs *fileservice.Service, path string) *fileservice.Folder {
	b.Helper()
	folder, err := fs.CreateFolder(ctx, path)
	if err != nil {
		b.Fatalf("CreateFolder failed: %v", err)
	}
	return folder
}

func generateBenchmarkData(size int) []byte {
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(rand.Intn(256))
	}

	return data
}

func BenchmarkCreateFile(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(benchmarkFileSize)

	b.SetBytes(int64(len(fileData)))
	for b.Loop() {
		path := fmt.Sprintf("bench_%s.txt", uuid.NewString())
		file := &fileservice.File{
			Path:        path,
			ContentType: "text/plain",
			Data:        fileData,
		}
		_, err := fileService.CreateFile(ctx, file)
		if err != nil {
			b.Fatalf("CreateFile failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkGetFileByID(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	const numFilesToPrepopulate = 100
	prePopulatedIDs := make([]string, 0, numFilesToPrepopulate)

	fileData := generateBenchmarkData(benchmarkFileSize)
	for i := range numFilesToPrepopulate {
		path := fmt.Sprintf("get_bench_%d.txt", i)
		createdFile := createFileForBenchmark(ctx, b, fileService, path, fileData, "text/plain")
		prePopulatedIDs = append(prePopulatedIDs, createdFile.ID)
	}
	// Create a file to retrieve
	createdFile := createFileForBenchmark(ctx, b, fileService, "get_me.txt", fileData, "text/plain")

	b.SetBytes(int64(len(fileData)))
	for b.Loop() {
		_, err := fileService.GetFileByID(ctx, createdFile.ID)
		if err != nil {
			b.Fatalf("GetFileByID failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkGetFilesByPath(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(benchmarkFileSize)
	// Create some files with the same path prefix
	basePath := "shared"
	createFileForBenchmark(ctx, b, fileService, filepath.Join(basePath, "file1.txt"), fileData, "text/plain")
	createFileForBenchmark(ctx, b, fileService, filepath.Join(basePath, "file2.txt"), fileData, "application/json")

	// We are listing metadata, not necessarily reading all the data in each iteration
	// Setting bytes here might be misleading.

	for b.Loop() {
		files, err := fileService.GetFilesByPath(ctx, basePath)
		if err != nil {
			b.Fatalf("GetFilesByPath failed: %v", err)
		}
		totalBytes := 0
		for _, f := range files {
			totalBytes += int(f.Size)
		}
		b.SetBytes(int64(totalBytes) / int64(len(files)))
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkUpdateFile(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(benchmarkFileSize)
	updatedData := generateBenchmarkData(benchmarkFileSize)
	// Create a file to update
	createdFile := createFileForBenchmark(ctx, b, fileService, "update_me.txt", fileData, "text/plain")

	b.SetBytes(int64(len(updatedData)))
	for b.Loop() {
		updatedFile := &fileservice.File{
			ID:          createdFile.ID,
			Path:        "update_me.txt",
			ContentType: "application/json",
			Data:        updatedData,
		}
		_, err := fileService.UpdateFile(ctx, updatedFile)
		if err != nil {
			b.Fatalf("UpdateFile failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkDeleteFile(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(benchmarkFileSize)

	// Deletion doesn't directly process bytes in the same way as read/write
	for i := 0; b.Loop(); i++ {
		// Create a new file for each iteration
		fileToDelete := createFileForBenchmark(ctx, b, fileService, fmt.Sprintf("delete_me_%d.txt", i), fileData, "text/plain")
		err := fileService.DeleteFile(ctx, fileToDelete.ID)
		if err != nil {
			b.Fatalf("DeleteFile failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkCreateFolder(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	for b.Loop() {
		path := fmt.Sprintf("folder_%s", uuid.NewString())
		_, err := fileService.CreateFolder(ctx, path)
		if err != nil {
			b.Fatalf("CreateFolder failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func populateFolderTree(ctx context.Context, b *testing.B, fs *fileservice.Service, basePath string, depth, breadth int, fileData []byte) {
	b.Helper()
	// Base case: if no more depth, just create a file and return.
	if depth <= 0 {
		filePath := filepath.Join(basePath, fmt.Sprintf("file_%s.txt", uuid.NewString()))
		createFileForBenchmark(ctx, b, fs, filePath, fileData, "text/plain")
		return
	}
	// Create a folder at the current level
	folderPath := filepath.Join(basePath, fmt.Sprintf("folder_%d", depth))
	createFolderForBenchmark(ctx, b, fs, folderPath)

	// Create some files in the current folder
	for i := range breadth {
		filePath := filepath.Join(folderPath, fmt.Sprintf("file_%d.txt", i))
		createFileForBenchmark(ctx, b, fs, filePath, fileData, "text/plain")
	}
	// Recursively create subfolders and files in each of them.
	for j := range breadth {
		subfolderPath := filepath.Join(folderPath, fmt.Sprintf("subfolder_%d", j))
		createFolderForBenchmark(ctx, b, fs, subfolderPath)
		// Recursively populate the subfolder tree
		populateFolderTree(ctx, b, fs, subfolderPath, depth-1, breadth, fileData)
	}
}

func BenchmarkRenameFolder(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(1024)
	createdFolder := createFolderForBenchmark(ctx, b, fileService, "old_folder")
	populateFolderTree(ctx, b, fileService, "old_folder/test", 3, 4, fileData)

	for b.Loop() {
		newPath := fmt.Sprintf("new_folder_%d", rand.Intn(1_000_000))
		_, err := fileService.RenameFolder(ctx, createdFolder.ID, newPath)
		if err != nil {
			b.Fatalf("RenameFolder failed: %v", err)
		}

		// Reset path
		_, err = fileService.RenameFolder(ctx, createdFolder.ID, "old_folder")
		if err != nil {
			b.Fatalf("Reset RenameFolder failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkListAllPaths(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(1024) // Using smaller data for many files

	// Create a realistic folder tree:
	// For example, 3 levels deep with 4 subfolders/files at each level
	populateFolderTree(ctx, b, fileService, "root", 3, 4, fileData)

	for b.Loop() {
		_, err := fileService.ListAllPaths(ctx)
		if err != nil {
			b.Fatalf("ListAllPaths failed: %v", err)
		}
	}
	showOpsPerSecond(b, int64(b.N))
}

func BenchmarkCreateFileParallel(b *testing.B) {
	ctx := context.Background()
	fileService, teardown := setupFileServiceBenchmark(ctx, b)
	defer teardown()

	fileData := generateBenchmarkData(benchmarkFileSize)
	b.ResetTimer()
	b.SetBytes(int64(len(fileData)))

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			path := fmt.Sprintf("bench_%s.txt", uuid.NewString())
			file := &fileservice.File{
				Path:        path,
				ContentType: "text/plain",
				Data:        fileData,
			}
			_, err := fileService.CreateFile(ctx, file)
			if err != nil {
				b.Fatalf("CreateFile failed: %v", err)
			}
		}
	})

	showOpsPerSecond(b, int64(b.N))
}
