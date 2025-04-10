package store_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

func TestAppendAndGetAllModels(t *testing.T) {
	ctx, s := store.SetupStore(t)

	models, err := s.ListModels(ctx)
	require.NoError(t, err)
	require.Empty(t, models)

	// Append a new model.
	model := &store.Model{
		ID:    uuid.New().String(),
		Model: "test-model",
	}
	err = s.AppendModel(ctx, model)
	require.NoError(t, err)
	require.NotEmpty(t, model.CreatedAt)
	require.NotEmpty(t, model.UpdatedAt)

	models, err = s.ListModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Equal(t, "test-model", models[0].Model)
	require.WithinDuration(t, model.CreatedAt, models[0].CreatedAt, time.Second)
	require.WithinDuration(t, model.UpdatedAt, models[0].UpdatedAt, time.Second)
}

func TestDeleteModel(t *testing.T) {
	ctx, s := store.SetupStore(t)

	model := &store.Model{
		ID:    uuid.New().String(),
		Model: "model-to-delete",
	}
	err := s.AppendModel(ctx, model)
	require.NoError(t, err)

	err = s.DeleteModel(ctx, "model-to-delete")
	require.NoError(t, err)

	models, err := s.ListModels(ctx)
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestDeleteNonExistentModel(t *testing.T) {
	ctx, s := store.SetupStore(t)

	err := s.DeleteModel(ctx, "non-existent-model")
	require.ErrorIs(t, err, libdb.ErrNotFound)
}

func TestGetAllModelsOrder(t *testing.T) {
	ctx, s := store.SetupStore(t)

	model1 := &store.Model{
		ID:    uuid.New().String(),
		Model: "model1",
	}
	err := s.AppendModel(ctx, model1)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)

	model2 := &store.Model{
		ID:    uuid.New().String(),
		Model: "model2",
	}
	err = s.AppendModel(ctx, model2)
	require.NoError(t, err)

	models, err := s.ListModels(ctx)
	require.NoError(t, err)
	require.Len(t, models, 2)
	require.Equal(t, "model2", models[0].Model)
	require.Equal(t, "model1", models[1].Model)
	require.True(t, models[0].CreatedAt.After(models[1].CreatedAt))
}

func TestAppendDuplicateModel(t *testing.T) {
	ctx, s := store.SetupStore(t)

	model := &store.Model{
		Model: "duplicate-model",
	}
	err := s.AppendModel(ctx, model)
	require.NoError(t, err)

	err = s.AppendModel(ctx, model)
	require.Error(t, err)
}
