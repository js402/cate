package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/js402/CATE/libs/libdb"
)

func (s *store) CreateUser(ctx context.Context, user *User) error {
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	err := s.Exec.QueryRowContext(ctx, `
		INSERT INTO users
		(id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		user.ID,
		user.FriendlyName,
		user.Email,
		user.Subject,
		user.HashedPassword,
		user.RecoveryCodeHash,
		user.Salt,
		user.CreatedAt,
		user.UpdatedAt,
	).Scan(&user.ID)
	if err != nil {
		return fmt.Errorf("%w: failed to create user", err)
	}
	return nil
}

func (s *store) GetUserByID(ctx context.Context, id string) (*User, error) {
	var user User
	err := s.Exec.QueryRowContext(ctx, `
		SELECT id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at
		FROM users
		WHERE id = $1`,
		id,
	).Scan(
		&user.ID,
		&user.FriendlyName,
		&user.Email,
		&user.Subject,
		&user.HashedPassword,
		&user.RecoveryCodeHash,
		&user.Salt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return &user, nil
}

func (s *store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	var user User
	err := s.Exec.QueryRowContext(ctx, `
		SELECT id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at
		FROM users
		WHERE email = $1`,
		email,
	).Scan(
		&user.ID,
		&user.FriendlyName,
		&user.Email,
		&user.Subject,
		&user.HashedPassword,
		&user.RecoveryCodeHash,
		&user.Salt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get user by email", err)
	}
	return &user, nil
}

func (s *store) GetUserBySubject(ctx context.Context, subject string) (*User, error) {
	var user User
	err := s.Exec.QueryRowContext(ctx, `
		SELECT id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at
		FROM users
		WHERE subject = $1`,
		subject,
	).Scan(
		&user.ID,
		&user.FriendlyName,
		&user.Email,
		&user.Subject,
		&user.HashedPassword,
		&user.RecoveryCodeHash,
		&user.Salt,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, libdb.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get user by subject", err)
	}
	return &user, nil
}

func (s *store) UpdateUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now().UTC()

	result, err := s.Exec.ExecContext(ctx, `
		UPDATE users
		SET
			friendly_name = $1,
			email = $2,
			subject = $3,
			hashed_password = $4,
			recovery_code_hash = $5,
			salt = $6,
			updated_at = $7
		WHERE id = $8`,
		user.FriendlyName,
		user.Email,
		user.Subject,
		user.HashedPassword,
		user.RecoveryCodeHash,
		user.Salt,
		user.UpdatedAt,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("%w : failed to update user", err)
	}

	return checkRowsAffected(result)
}

func (s *store) DeleteUser(ctx context.Context, id string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM users
		WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("%w: failed to delete user", err)
	}
	return checkRowsAffected(result)
}

func (s *store) ListUsers(ctx context.Context, createdAtCursor time.Time) ([]*User, error) {
	rows, err := s.Exec.QueryContext(ctx, `
		SELECT id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at
		FROM users WHERE created_at <= $1
		ORDER BY created_at DESC LIMIT 10000`, createdAtCursor,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query users", err)
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.FriendlyName,
			&user.Email,
			&user.Subject,
			&user.HashedPassword,
			&user.RecoveryCodeHash,
			&user.Salt,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%w: failed to scan user", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: rows iteration error", err)
	}

	return users, nil
}

func (s *store) ListUsersBySubjects(ctx context.Context, subjects ...string) ([]*User, error) {
	if len(subjects) == 0 {
		return []*User{}, nil
	}
	q := ""
	args := make([]any, 0, len(subjects)+1)
	for i, v := range subjects {
		q += fmt.Sprintf("$%d", i+1)
		if i < len(subjects)-1 {
			q += ", "
		}
		args = append(args, v)
	}

	query := fmt.Sprintf(`
		SELECT id, friendly_name, email, subject, hashed_password, recovery_code_hash, salt, created_at, updated_at
		FROM users
		WHERE subject IN (%s) ORDER BY created_at DESC`, q)

	rows, err := s.Exec.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to query users", err)
	}
	defer rows.Close()

	users := []*User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(
			&user.ID,
			&user.FriendlyName,
			&user.Email,
			&user.Subject,
			&user.HashedPassword,
			&user.RecoveryCodeHash,
			&user.Salt,
			&user.CreatedAt,
			&user.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("%w: failed to scan user", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: rows iteration error", err)
	}

	return users, nil
}
