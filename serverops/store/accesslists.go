package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/js402/cate/libs/libdb"
)

// CreateAccessEntry creates a new access control entry
func (s *store) CreateAccessEntry(ctx context.Context, entry *AccessEntry) error {
	now := time.Now().UTC()
	entry.CreatedAt = now
	entry.UpdatedAt = now

	err := s.Exec.QueryRowContext(ctx, `
		INSERT INTO accesslists
		(id, identity, resource, permission, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		entry.ID,
		entry.Identity,
		entry.Resource,
		entry.Permission,
		entry.CreatedAt,
		entry.UpdatedAt,
	).Scan(&entry.ID)
	if err != nil {
		return fmt.Errorf("failed to create access entry: %w", err)
	}
	return nil
}

// GetAccessEntryByID retrieves an entry by its UUID
func (s *store) GetAccessEntryByID(ctx context.Context, id string) (*AccessEntry, error) {
	var entry AccessEntry
	err := s.Exec.QueryRowContext(ctx, `
		SELECT id, identity, resource, permission, created_at, updated_at
		FROM accesslists
		WHERE id = $1`,
		id,
	).Scan(
		&entry.ID,
		&entry.Identity,
		&entry.Resource,
		&entry.Permission,
		&entry.CreatedAt,
		&entry.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get access entry by ID: %w", err)
	}
	return &entry, nil
}

// UpdateAccessEntry updates an existing access control entry
func (s *store) UpdateAccessEntry(ctx context.Context, entry *AccessEntry) error {
	entry.UpdatedAt = time.Now().UTC()

	result, err := s.Exec.ExecContext(ctx, `
		UPDATE accesslists
		SET
			identity = $1,
			resource = $2,
			permission = $3,
			updated_at = $4
		WHERE id = $5`,
		entry.Identity,
		entry.Resource,
		entry.Permission,
		entry.UpdatedAt,
		entry.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update access entry: %w", err)
	}

	return checkRowsAffected(result)
}

// DeleteAccessEntry removes an access control entry by ID
func (s *store) DeleteAccessEntry(ctx context.Context, id string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM accesslists
		WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to delete access entry: %w", err)
	}
	return checkRowsAffected(result)
}

func (s *store) DeleteAccessEntriesByIdentity(ctx context.Context, identity string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM accesslists
		WHERE identity = $1`,
		identity,
	)
	if err != nil {
		return fmt.Errorf("failed to delete access entry: %w", err)
	}
	return checkRowsAffected(result)
}

func (s *store) DeleteAccessEntriesByResource(ctx context.Context, resource string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM accesslists
		WHERE resource = $1`,
		resource,
	)
	if err != nil {
		return fmt.Errorf("failed to delete access entry: %w", err)
	}
	return checkRowsAffected(result)
}

// ListAccessEntries returns all access control entries ordered by creation time
func (s *store) ListAccessEntries(ctx context.Context, createdAtCursor time.Time) ([]*AccessEntry, error) {
	rows, err := s.Exec.QueryContext(ctx, `
		SELECT id, identity, resource, permission, created_at, updated_at
		FROM accesslists
		WHERE created_at <= $1
		ORDER BY created_at DESC LIMIT 10000`, createdAtCursor,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query access entries: %w", err)
	}
	defer rows.Close()

	entries := []*AccessEntry{}
	for rows.Next() {
		var entry AccessEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.Identity,
			&entry.Resource,
			&entry.Permission,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan access entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return entries, nil
}

// GetAccessEntriesByIdentity returns all entries for a specific identity
func (s *store) GetAccessEntriesByIdentity(ctx context.Context, identity string) ([]*AccessEntry, error) {
	rows, err := s.Exec.QueryContext(ctx, `
		SELECT id, identity, resource, permission, created_at, updated_at
		FROM accesslists
		WHERE identity = $1
		ORDER BY created_at DESC LIMIT 10000`,
		identity,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query access entries by identity: %w", err)
	}
	defer rows.Close()

	entries := []*AccessEntry{}
	for rows.Next() {
		var entry AccessEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.Identity,
			&entry.Resource,
			&entry.Permission,
			&entry.CreatedAt,
			&entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan access entry: %w", err)
		}
		entries = append(entries, &entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return entries, nil
}
