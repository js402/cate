package fileservice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/js402/cate/libs/libdb"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/serverops/store"
)

const MaxUploadSize = 1 * 1024 * 1024
const MaxFilesRowCount = 50000

type Service struct {
	db libdb.DBManager
}

func New(db libdb.DBManager, config *serverops.Config) *Service {
	return &Service{
		db: db,
	}
}

// File represents a file entity.
type File struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	Data        []byte `json:"data"`
}

type Folder struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

// Metadata holds file metadata.
type Metadata struct {
	SpecVersion string `json:"specVersion"`
	Path        string `json:"path"`
	Hash        string `json:"hash"`
	Size        int64  `json:"size"`
	FileID      string `json:"fileId"`
}

func (s *Service) CreateFile(ctx context.Context, file *File) (*File, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	if err := validateContentType(file.ContentType); err != nil {
		return nil, fmt.Errorf("invalid content type: %w", err)
	}
	cleanedPath, err := sanitizePath(file.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	mediaType, _, parseMediaErr := mime.ParseMediaType(file.ContentType)
	if parseMediaErr != nil {
		err = fmt.Errorf("invalid detected content type '%s': %v", file.ContentType, parseMediaErr)
		return nil, err
	}
	if mediaType != file.ContentType {
		return nil, fmt.Errorf("invalid detected content type. Claimed as '%s' yet is %v", file.ContentType, mediaType)
	}
	if !allowedMimeTypes[mediaType] {
		err = fmt.Errorf("MIME type '%s' (detected: %s) is not allowed for file '%s'", mediaType, file.ContentType, file.Path)
		return nil, err
	}
	file.Path = cleanedPath

	// Generate IDs.
	fileID := uuid.NewString()
	blobID := uuid.NewString()

	// Compute SHA-256 hash of the file data.
	hashBytes := sha256.Sum256(file.Data)
	hashString := hex.EncodeToString(hashBytes[:])

	meta := Metadata{
		SpecVersion: "1.0",
		Path:        file.Path,
		Hash:        hashString,
		Size:        int64(len(file.Data)),
		FileID:      fileID,
	}
	bMeta, err := json.Marshal(&meta)
	if err != nil {
		return nil, err
	}

	// Create blob record.
	blob := &store.Blob{
		ID:   blobID,
		Data: file.Data,
		Meta: bMeta,
	}
	if file.Size > MaxUploadSize {
		return nil, serverops.ErrFileSizeLimitExceeded
	}
	// Start a transaction.
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}
	storeService := store.New(tx)
	err = storeService.EnforceMaxFileCount(ctx, MaxFilesRowCount)
	if err != nil {
		err := fmt.Errorf("too many files in the system: %w", err)
		fmt.Printf("SERVER ERROR: file creation blocked: limit reached (%d max) %v", err, MaxFilesRowCount)
		return nil, err
	}

	if err = storeService.CreateBlob(ctx, blob); err != nil {
		return nil, fmt.Errorf("failed to create blob: %w", err)
	}

	// Create file record.
	fileRecord := &store.File{
		ID:      fileID,
		Path:    file.Path,
		Type:    file.ContentType,
		Meta:    bMeta,
		BlobsID: blobID,
	}
	if err = storeService.CreateFile(ctx, fileRecord); err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	creatorID, err := serverops.GetIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}
	if creatorID == "" {
		return nil, fmt.Errorf("creator ID is empty")
	}
	// Grant access to the creator.
	accessEntry := &store.AccessEntry{
		ID:         uuid.NewString(),
		Identity:   creatorID,
		Resource:   fileID,
		Permission: store.PermissionManage,
	}
	if err := storeService.CreateAccessEntry(ctx, accessEntry); err != nil {
		return nil, fmt.Errorf("failed to create access entry: %w", err)
	}
	resFiles, err := s.getFileByID(ctx, tx, fileID)
	if err != nil {
		return nil, err
	}
	err = commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return resFiles, nil
}

func (s *Service) GetFileByID(ctx context.Context, id string) (*File, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	// Start a transaction.
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}
	resFile, err := s.getFileByID(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx); err != nil {
		return nil, err
	}
	return resFile, nil
}

func (s *Service) getFileByID(ctx context.Context, tx libdb.Exec, id string) (*File, error) {
	// Get file record.
	fileRecord, err := store.New(tx).GetFileByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := serverops.CheckResourceAuthorization(ctx, fileRecord.ID, store.PermissionView); err != nil {
		return nil, err
	}
	// Get associated blob.
	blob, err := store.New(tx).GetBlobByID(ctx, fileRecord.BlobsID)
	if err != nil {
		return nil, err
	}

	// Reconstruct the File.
	resFile := &File{
		ID:          fileRecord.ID,
		Path:        fileRecord.Path,
		ContentType: fileRecord.Type,
		Data:        blob.Data,
		Size:        int64(len(blob.Data)),
	}

	return resFile, nil
}

func (s *Service) GetFilesByPath(ctx context.Context, path string) ([]File, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	// Start a transaction to fetch files and their blobs.
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}

	fileRecords, err := store.New(tx).ListFilesByPath(ctx, path)
	if err != nil {
		return nil, err
	}

	var files []File
	for _, fr := range fileRecords {
		blob, err := store.New(tx).GetBlobByID(ctx, fr.BlobsID)
		if err != nil {
			return nil, err
		}
		files = append(files, File{
			ID:          fr.ID,
			Path:        fr.Path,
			ContentType: fr.Type,
			//Data:        blob.Data, // Don't include data in response without permission check
			Size: int64(len(blob.Data)),
		})
	}

	if err := commit(ctx); err != nil {
		return nil, err
	}
	return files, nil
}

func (s *Service) UpdateFile(ctx context.Context, file *File) (*File, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	if err := validateContentType(file.ContentType); err != nil {
		return nil, fmt.Errorf("invalid content type: %w", err)
	}
	cleanedPath, err := sanitizePath(file.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	file.Path = cleanedPath

	mediaType, _, parseMediaErr := mime.ParseMediaType(file.ContentType)
	if parseMediaErr != nil {
		err = fmt.Errorf("invalid detected content type '%s': %v", file.ContentType, parseMediaErr)
		return nil, err
	}
	if mediaType != file.ContentType {
		return nil, fmt.Errorf("invalid detected content type. Claimed as '%s' yet is %v", file.ContentType, mediaType)
	}
	if !allowedMimeTypes[mediaType] {
		err = fmt.Errorf("MIME type '%s' (detected: %s) is not allowed for file '%s'", mediaType, file.ContentType, file.Path)
		return nil, err
	}
	// Start a transaction.
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}

	// Retrieve the existing file record to get the blob ID.
	existing, err := store.New(tx).GetFileByID(ctx, file.ID)
	if err != nil {
		return nil, err
	}
	if err := serverops.CheckResourceAuthorization(ctx, existing.ID, store.PermissionEdit); err != nil {
		return nil, err
	}
	blobID := existing.BlobsID

	// Compute new hash and metadata.
	hashBytes := sha256.Sum256(file.Data)
	hashString := hex.EncodeToString(hashBytes[:])
	meta := Metadata{
		SpecVersion: "1.0",
		Path:        existing.Path,
		Hash:        hashString,
		Size:        int64(len(file.Data)),
		FileID:      file.ID,
	}
	bMeta, err := json.Marshal(&meta)
	if err != nil {
		return nil, err
	}

	// Update blob record.
	blob := &store.Blob{
		ID:   blobID,
		Data: file.Data,
		Meta: bMeta,
	}
	if err := store.New(tx).DeleteBlob(ctx, blobID); err != nil {
		return nil, fmt.Errorf("failed to delete blob: %w", err)
	}
	if err := store.New(tx).CreateBlob(ctx, blob); err != nil {
		return nil, fmt.Errorf("failed to update blob: %w", err)
	}

	// Update file record.
	updatedFile := &store.File{
		ID:      file.ID,
		Path:    file.Path,
		Type:    file.ContentType,
		Meta:    bMeta,
		BlobsID: blobID,
	}
	if err := store.New(tx).UpdateFile(ctx, updatedFile); err != nil {
		return nil, fmt.Errorf("failed to update file: %w", err)
	}

	resFile, err := s.getFileByID(ctx, tx, file.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	err = commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	return resFile, nil
}

func (s *Service) DeleteFile(ctx context.Context, id string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return err
	}
	storeService := store.New(tx)

	// Get file details.
	file, err := storeService.GetFileByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}
	if err := serverops.CheckResourceAuthorization(ctx, file.ID, store.PermissionManage); err != nil {
		return err
	}
	// Delete associated blob.
	if err := storeService.DeleteBlob(ctx, file.BlobsID); err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}

	// Delete file record.
	if err := storeService.DeleteFile(ctx, id); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Remove related access entries.
	if err := storeService.DeleteAccessEntriesByResource(ctx, id); err != nil {
		return fmt.Errorf("failed to delete access entries: %w", err)
	}

	return commit(ctx)
}

func (s *Service) ListAllPaths(ctx context.Context) ([]string, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}
	// Start a transaction.
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}

	// Retrieve the distinct paths using the store method.
	paths, err := store.New(tx).ListFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all paths: %w", err)
	}

	// Commit the transaction.
	if err := commit(ctx); err != nil {
		return nil, err
	}
	return paths, nil
}

func (s *Service) CreateFolder(ctx context.Context, path string) (*Folder, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}

	cleanedPath, err := sanitizePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Generate folder ID
	folderID := uuid.NewString()

	// Create metadata
	meta := Metadata{
		SpecVersion: "1.0",
		Path:        cleanedPath,
		FileID:      folderID,
		// Hash and Size are omitted for folders
	}
	bMeta, err := json.Marshal(&meta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	// Start transaction
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer func() {
		if err := rTx(); err != nil {
			log.Println("failed to rollback transaction", err)
		}
	}()
	if err != nil {
		return nil, err
	}

	storeService := store.New(tx)

	// Enforce max file count (includes folders)
	if err := storeService.EnforceMaxFileCount(ctx, MaxFilesRowCount); err != nil {
		return nil, fmt.Errorf("too many files in the system: %w", err)
	}

	// Create folder record
	folderRecord := &store.File{
		ID:       folderID,
		Path:     cleanedPath,
		Meta:     bMeta,
		IsFolder: true,
	}

	if err := storeService.CreateFile(ctx, folderRecord); err != nil {
		return nil, fmt.Errorf("failed to create folder: %w", err)
	}

	if err := commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &Folder{
		ID:   folderID,
		Path: cleanedPath,
	}, nil
}

func (s *Service) RenameFile(ctx context.Context, fileID, newPath string) (*File, error) {
	// Check resource-level edit permission
	if err := serverops.CheckResourceAuthorization(ctx, fileID, store.PermissionEdit); err != nil {
		return nil, err
	}

	// Start transaction
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer rTx()
	if err != nil {
		return nil, err
	}
	storeService := store.New(tx)

	// Get the file
	fileRecord, err := storeService.GetFileByID(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("file not found: %w", err)
	}
	if fileRecord.IsFolder {
		return nil, fmt.Errorf("target is a folder, use RenameFolder instead")
	}

	// Sanitize and validate new path
	cleanedPath, err := sanitizePath(newPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check for existing file/folder at new path
	existing, err := storeService.ListFilesByPath(ctx, cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check path availability: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("path '%s' already exists", cleanedPath)
	}

	// Update file path
	if err := storeService.UpdateFilePath(ctx, fileID, cleanedPath); err != nil {
		return nil, fmt.Errorf("failed to rename file: %w", err)
	}

	// Commit transaction
	if err := commit(ctx); err != nil {
		return nil, err
	}

	// Return updated file
	return s.GetFileByID(ctx, fileID)
}

func (s *Service) RenameFolder(ctx context.Context, folderID, newPath string) (*Folder, error) {
	// Check service-level manage permission
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return nil, err
	}

	// Start transaction
	tx, commit, rTx, err := s.db.WithTransaction(ctx)
	defer rTx()
	if err != nil {
		return nil, err
	}
	storeService := store.New(tx)

	// Get the folder
	folderRecord, err := storeService.GetFileByID(ctx, folderID)
	if err != nil {
		return nil, fmt.Errorf("folder not found: %w", err)
	}
	if !folderRecord.IsFolder {
		return nil, fmt.Errorf("target is not a folder")
	}
	oldPath := folderRecord.Path

	// Sanitize and validate new path
	cleanedPath, err := sanitizePath(newPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Check for existing file/folder at new path
	existing, err := storeService.ListFilesByPath(ctx, cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check path availability: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("path '%s' already exists", cleanedPath)
	}

	// Update folder's own path
	if err := storeService.UpdateFilePath(ctx, folderID, cleanedPath); err != nil {
		return nil, fmt.Errorf("failed to rename folder: %w", err)
	}

	// List all files under the old folder path (prefix match)
	descendants, err := storeService.ListFilesByPath(ctx, oldPath+"/%")
	if err != nil {
		return nil, fmt.Errorf("failed to list folder contents: %w", err)
	}

	// Prepare bulk updates (ID -> new path)
	updates := make(map[string]string)
	for _, file := range descendants {
		newFilePath := strings.Replace(file.Path, oldPath, cleanedPath, 1)
		updates[file.ID] = newFilePath
	}

	// Apply bulk updates
	if err := storeService.BulkUpdateFilePaths(ctx, updates); err != nil {
		return nil, fmt.Errorf("failed to update descendant paths: %w", err)
	}

	// Commit transaction
	if err := commit(ctx); err != nil {
		return nil, err
	}

	return &Folder{
		ID:   folderID,
		Path: cleanedPath,
	}, nil
}

func (s *Service) GetServiceName() string {
	return "fileservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}

func detectMimeTee(r io.Reader) (string, io.Reader, error) {
	buf := make([]byte, 512)
	tee := io.TeeReader(r, bytes.NewBuffer(buf[:0]))
	_, err := io.ReadFull(tee, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", nil, err
	}
	mimeType := http.DetectContentType(buf)

	// Rebuild a combined reader: first the sniffed bytes, then the rest
	combined := io.MultiReader(bytes.NewReader(buf), r)
	return mimeType, combined, nil
}

func detectMimeTypeFromReader(r io.Reader) (string, []byte, error) {
	buffer := make([]byte, 512)
	n, err := r.Read(buffer)
	if err != nil && err != io.EOF {
		return "", nil, err
	}

	mimeType := http.DetectContentType(buffer[:n])

	// reassemble the remaining content
	remaining, err := io.ReadAll(r)
	if err != nil {
		return "", nil, err
	}

	// Combine the sniffed part and the rest
	fullContent := append(buffer[:n], remaining...)
	return mimeType, fullContent, nil
}

var allowedMimeTypes = map[string]bool{
	"text/plain":       true,
	"application/json": true,
	"application/pdf":  true,
}

func validateContentType(contentType string) error {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("invalid content type: %v", err)
	}
	if !allowedMimeTypes[mediaType] {
		return fmt.Errorf("content type %s is not allowed", mediaType)
	}
	return nil
}

func sanitizePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path contains parent directory traversal")
	}
	return cleaned, nil
}
