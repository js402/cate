package filesapi

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/services/fileservice"
)

const (
	MaxRequestSize      = fileservice.MaxUploadSize + 10*1024
	multipartFormMemory = 8 << 20
	formFieldFile       = "file"
	formFieldPath       = "path"
)

func AddFileRoutes(mux *http.ServeMux, config *serverops.Config, fileService *fileservice.Service) {
	f := &fileManager{
		service: fileService,
	}

	mux.HandleFunc("POST /files", f.create)
	mux.HandleFunc("GET /files/{id}", f.getMetadata)
	mux.HandleFunc("PUT /files/{id}", f.update)
	mux.HandleFunc("DELETE /files/{id}", f.delete)
	mux.HandleFunc("GET /files/{id}/download", f.download)
	mux.HandleFunc("GET /files/paths", f.listPaths)
}

type fileManager struct {
	service *fileservice.Service
}

type fileResponse struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func mapFileToResponse(f *fileservice.File) fileResponse {
	return fileResponse{
		ID:          f.ID,
		Path:        f.Path,
		ContentType: f.ContentType,
		Size:        f.Size,
	}
}

// It validates the request, size, and MIME type. Reads the file content into memory,
// ensuring not to read more than MaxUploadSize bytes even if headers are manipulated.
// Returns file header, full file data ([]byte), path, detected mimeType, and error using unnamed returns.
func (f *fileManager) processAndReadFileUpload(w http.ResponseWriter, r *http.Request) (
	*multipart.FileHeader, // header
	[]byte, // fileData
	string, // path
	string, // mimeType
	error, // err
) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)

	if parseErr := r.ParseMultipartForm(multipartFormMemory); parseErr != nil {
		var localErr error
		var maxBytesErr *http.MaxBytesError
		if errors.As(parseErr, &maxBytesErr) {
			localErr = fmt.Errorf("request body too large (limit %d bytes): %w", maxBytesErr.Limit, parseErr)
		} else if errors.Is(parseErr, http.ErrNotMultipart) {
			localErr = fmt.Errorf("invalid request format (not multipart): %w", parseErr)
		} else {
			localErr = fmt.Errorf("failed to parse multipart form: %w", parseErr)
		}
		return nil, nil, "", "", localErr
	}

	filePart, header, formErr := r.FormFile(formFieldFile)
	if formErr != nil {
		if errors.Is(formErr, http.ErrMissingFile) {
			return nil, nil, "", "", formErr
		}
		localErr := fmt.Errorf("invalid '%s' upload: %w", formFieldFile, formErr)
		return nil, nil, "", "", localErr
	}
	defer filePart.Close()

	// a quick check.
	if header.Size > fileservice.MaxUploadSize {
		return nil, nil, "", "", serverops.ErrFileSizeLimitExceeded
	}
	if header.Size == 0 {
		return nil, nil, "", "", serverops.ErrFileEmpty
	}

	// Reading one extra byte allows us to detect if the original file was larger than the limit.
	limitedReader := io.LimitReader(filePart, fileservice.MaxUploadSize+1)
	fileData, readErr := io.ReadAll(limitedReader)
	if readErr != nil {
		localErr := fmt.Errorf("failed to read file content for '%s': %w", header.Filename, readErr)
		return nil, nil, "", "", localErr
	}

	// If we read more than MaxUploadSize bytes, it means the original stream had more data.
	if int64(len(fileData)) > fileservice.MaxUploadSize {
		return nil, nil, "", "", serverops.ErrFileSizeLimitExceeded
	}
	// We now have the file data, guaranteed to be <= MaxUploadSize bytes.

	detectedMimeType := http.DetectContentType(fileData)

	var resultPath string
	specifiedPath := r.FormValue(formFieldPath)
	if specifiedPath == "" {
		resultPath = header.Filename
	} else {
		resultPath = specifiedPath
	}

	return header, fileData, resultPath, detectedMimeType, nil
}

// create handles the creation of a new file using multipart/form-data.
func (f *fileManager) create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	header, fileData, path, mimeType, err := f.processAndReadFileUpload(w, r)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	req := fileservice.File{
		Path:        path,
		ContentType: mimeType,
		Data:        fileData,
		Size:        header.Size,
	}

	file, err := f.service.CreateFile(ctx, &req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.CreateOperation)
		return
	}

	_ = serverops.Encode(w, r, http.StatusCreated, file)
}

// getMetadata - No change needed
func (f *fileManager) getMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	file, err := f.service.GetFileByID(ctx, id)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}

	serverops.Encode(w, r, http.StatusOK, mapFileToResponse(file))
}

// update handles updating an existing file using multipart/form-data.
func (f *fileManager) update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	header, fileData, path, mimeType, err := f.processAndReadFileUpload(w, r)
	if err != nil {
		// Pass the raw error to serverops.Error
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	req := fileservice.File{
		ID:          id,
		Path:        path,
		ContentType: mimeType,
		Data:        fileData,
		Size:        header.Size,
	}

	file, err := f.service.UpdateFile(ctx, &req)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.UpdateOperation)
		return
	}

	serverops.Encode(w, r, http.StatusOK, file)
}

// delete - No change needed
func (f *fileManager) delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	if err := f.service.DeleteFile(ctx, id); err != nil {
		_ = serverops.Error(w, r, err, serverops.DeleteOperation)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listPaths - No change needed
func (f *fileManager) listPaths(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	paths, err := f.service.ListAllPaths(ctx)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.ListOperation)
		return
	}
	if paths == nil {
		paths = []string{}
	}
	_ = serverops.Encode(w, r, http.StatusOK, paths)
}

// download streams the file content.
func (f *fileManager) download(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	skip := r.URL.Query().Get("skip")

	file, err := f.service.GetFileByID(ctx, id)
	if err != nil {
		_ = serverops.Error(w, r, err, serverops.GetOperation)
		return
	}
	if file == nil {
		_ = serverops.Error(w, r, fmt.Errorf("file with id '%s' not found", id), serverops.GetOperation)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	sanitizedFilename := strconv.Quote(file.Path)
	if skip != "true" {
		w.Header().Set("Content-Disposition", "attachment; filename="+sanitizedFilename)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(file.Size, 10))

	_, copyErr := bytes.NewReader(file.Data).WriteTo(w)
	if copyErr != nil {
		// Can't do much here if writing to response fails midway
	}
}
