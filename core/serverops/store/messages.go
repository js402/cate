package store

import (
	"context"
	"fmt"
	"time"
)

func (s *store) AppendMessage(ctx context.Context, message *Message) error {
	now := time.Now().UTC()
	message.AddedAt = now

	_, err := s.Exec.ExecContext(ctx, `
		INSERT INTO messages
		(id, stream, payload, added_at)
		VALUES ($1, $2, $3, $4)`,
		message.ID,
		message.Stream,
		message.Payload,
		message.AddedAt,
	)
	return err
}

func (s *store) DeleteMessages(ctx context.Context, stream string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM messages
		WHERE stream = $1`,
		stream,
	)

	if err != nil {
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *store) ListMessages(ctx context.Context, stream string) ([]*Message, error) {
	rows, err := s.Exec.QueryContext(ctx, `
		SELECT id, stream, payload, added_at
		FROM messages
		WHERE stream = $1
		ORDER BY added_at DESC`,
		stream,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	models := []*Message{}
	for rows.Next() {
		var model Message
		if err := rows.Scan(
			&model.ID,
			&model.Stream,
			&model.Payload,
			&model.AddedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan messages: %w", err)
		}
		models = append(models, &model)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return models, nil
}
