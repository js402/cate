package store

import (
	"context"
	"fmt"
	"time"
)

func (s *store) AppendModel(ctx context.Context, model *Model) error {
	now := time.Now().UTC()
	model.CreatedAt = now
	model.UpdatedAt = now

	_, err := s.Exec.ExecContext(ctx, `
		INSERT INTO ollama_models
		(id, model, created_at, updated_at)
		VALUES ($1, $2, $3, $4)`,
		model.ID,
		model.Model,
		model.CreatedAt,
		model.UpdatedAt,
	)
	return err
}

func (s *store) DeleteModel(ctx context.Context, modelName string) error {
	result, err := s.Exec.ExecContext(ctx, `
		DELETE FROM ollama_models
		WHERE model = $1`,
		modelName,
	)

	if err != nil {
		return fmt.Errorf("failed to delete model: %w", err)
	}

	return checkRowsAffected(result)
}

func (s *store) ListModels(ctx context.Context) ([]*Model, error) {
	rows, err := s.Exec.QueryContext(ctx, `
		SELECT id, model, created_at, updated_at
		FROM ollama_models
		ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer rows.Close()

	models := []*Model{}
	for rows.Next() {
		var model Model
		if err := rows.Scan(
			&model.ID,
			&model.Model,
			&model.CreatedAt,
			&model.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}
		models = append(models, &model)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return models, nil
}
