package store_test

import (
	"context"
	"testing"

	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libdb"
	"github.com/stretchr/testify/require"
)

func TestQueryingEmptyDB(t *testing.T) {
	ctx := context.TODO()
	connStr, _, cleanup, err := libdb.SetupLocalInstance(ctx, "test", "test", "test")
	require.NoError(t, err)
	dbManager, err := libdb.NewPostgresDBManager(ctx, connStr, store.Schema)
	require.NoError(t, err)
	_ = store.New(dbManager.WithoutTransaction())
	t.Cleanup(func() {
		err := dbManager.Close()
		require.NoError(t, err)

		cleanup()
	})
}
