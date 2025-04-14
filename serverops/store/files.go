package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/js402/cate/libs/libdb"
)

func (s *store) CreateFile(ctx context.Context, file *File) error {
	now := time.Now().UTC()
	file.CreatedAt = now
	file.UpdatedAt = now
	_, err := s.Exec.ExecContext(ctx, `
        INSERT INTO files
        (id, path, type, meta, blobs_id, is_folder, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		file.ID,
		file.Path,
		file.Type,
		file.Meta,
		file.BlobsID,
		file.IsFolder,
		file.CreatedAt,
		file.UpdatedAt,
	)
	return err
}

func (s *store) GetFileByID(ctx context.Context, id string) (*File, error) {
	var file File
	err := s.Exec.QueryRowContext(ctx, `
        SELECT id, path, type, meta, blobs_id, is_folder, created_at, updated_at
        FROM files
        WHERE id = $1`,
		id,
	).Scan(
		&file.ID,
		&file.Path,
		&file.Type,
		&file.Meta,
		&file.BlobsID,
		&file.IsFolder,
		&file.CreatedAt,
		&file.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	return &file, err
}

func (s *store) ListFilesByPath(ctx context.Context, path string) ([]File, error) {
	rows, err := s.Exec.QueryContext(ctx, `
        SELECT id, path, type, meta, blobs_id, is_folder, created_at, updated_at
        FROM files
        WHERE path LIKE $1`,
		path+"%",
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []File
	for rows.Next() {
		var file File
		if err := rows.Scan(
			&file.ID,
			&file.Path,
			&file.Type,
			&file.Meta,
			&file.BlobsID,
			&file.IsFolder,
			&file.CreatedAt,
			&file.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		files = append(files, file)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return files, nil
}

func (s *store) UpdateFile(ctx context.Context, file *File) error {
	file.UpdatedAt = time.Now().UTC()

	result, err := s.Exec.ExecContext(ctx, `
        UPDATE files
        SET path = $2,
            type = $3,
            meta = $4,
            is_folder = $5,
            blobs_id = $6,
            updated_at = $7
        WHERE id = $1`,
		file.ID,
		file.Path,
		file.Type,
		file.Meta,
		file.IsFolder,
		file.BlobsID,
		file.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update file: %w", err)
	}
	return checkRowsAffected(result)
}

func (s *store) UpdateFilePath(ctx context.Context, id string, newPath string) error {
	now := time.Now().UTC()

	result, err := s.Exec.ExecContext(ctx, `
		UPDATE files
		SET path = $2,
			updated_at = $3
		WHERE id = $1`,
		id,
		newPath,
		now,
	)
	if err != nil {
		return err
	}

	return checkRowsAffected(result)
}

func (s *store) BulkUpdateFilePaths(ctx context.Context, updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}
	now := time.Now().UTC()
	sqlPrefix := "UPDATE files SET updated_at = $1, path = CASE id "
	sqlSuffix := " END WHERE id IN ("
	args := []any{now}
	ids := []string{}
	i := 2

	for id, newPath := range updates {
		sqlPrefix += fmt.Sprintf("WHEN $%d THEN $%d ", i, i+1)
		args = append(args, id, newPath)
		ids = append(ids, fmt.Sprintf("$%d", i))
		i += 2
	}
	sqlPrefix += "ELSE path" // to avoid avoid SQL errors, not be strictly necessary with WHERE IN

	// Build WHERE IN clause placeholders ($N, $N+1, ...)
	placeholders := ""
	for j := range len(updates) {
		placeholders += fmt.Sprintf("$%d", j*2+2) // Reference the ID args
		if j < len(updates)-1 {
			placeholders += ","
		}
	}

	finalSQL := sqlPrefix + sqlSuffix + placeholders + ")"

	result, err := s.Exec.ExecContext(ctx, finalSQL, args...)
	if err != nil {
		return fmt.Errorf("failed bulk update paths with CASE: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *store) DeleteFile(ctx context.Context, id string) error {
	result, err := s.Exec.ExecContext(ctx, `
        DELETE FROM files
        WHERE id = $1`,
		id,
	)

	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return checkRowsAffected(result)
}

func (s *store) ListFiles(ctx context.Context) ([]string, error) {
	rows, err := s.Exec.QueryContext(ctx, `
        SELECT DISTINCT path FROM files
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to list paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("failed to scan path: %w", err)
		}
		paths = append(paths, path)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}
	return paths, nil
}

func (s *store) EstimateFileCount(ctx context.Context) (int64, error) {
	var count int64
	err := s.Exec.QueryRowContext(ctx, `
		SELECT estimate_row_count('files')
	`).Scan(&count)
	return count, err
}

func (s *store) EnforceMaxFileCount(ctx context.Context, maxCount int64) error {
	count, err := s.EstimateFileCount(ctx)
	if err != nil {
		return err
	}
	if count >= maxCount {
		return fmt.Errorf("file limit reached (max 60,000)")
	}
	return nil
}
