package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/js402/cate/libs/libdb"
)

func (s *store) CreateBlob(ctx context.Context, blob *Blob) error {
	now := time.Now().UTC()
	blob.CreatedAt = now
	blob.UpdatedAt = now

	_, err := s.Exec.ExecContext(ctx, `
        INSERT INTO blobs
        (id, meta, data, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5)`,
		blob.ID,
		blob.Meta,
		blob.Data,
		blob.CreatedAt,
		blob.UpdatedAt,
	)
	return err
}

func (s *store) GetBlobByID(ctx context.Context, id string) (*Blob, error) {
	var blob Blob
	err := s.Exec.QueryRowContext(ctx, `
        SELECT id, meta, data, created_at, updated_at
        FROM blobs
        WHERE id = $1`,
		id,
	).Scan(
		&blob.ID,
		&blob.Meta,
		&blob.Data,
		&blob.CreatedAt,
		&blob.UpdatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	return &blob, err
}

func (s *store) DeleteBlob(ctx context.Context, id string) error {
	result, err := s.Exec.ExecContext(ctx, `
        DELETE FROM blobs
        WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to delete blob: %w", err)
	}
	return checkRowsAffected(result)
}
