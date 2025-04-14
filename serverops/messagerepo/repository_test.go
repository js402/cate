package messagerepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/cate/serverops/messagerepo"
	"github.com/stretchr/testify/require"
)

func TestCRUD(t *testing.T) {
	// Initialize a new test repository backed by an OpenSearch test container.
	repo, cleanup, err := messagerepo.NewTestStore(t)
	require.NoError(t, err, "failed to initialize test repository")
	defer cleanup()

	ctx := context.Background()

	// Create a new message.
	msg := messagerepo.Message{
		ID:          uuid.NewString(),
		MessageID:   uuid.NewString(),
		SpecVersion: "1.0",
		Type:        "chat",
		Time:        time.Now(),
		Subject:     "testingstream",
		Source:      "tests",
		Data:        `{"example":"data"}`,
		ReceivedAt:  time.Now(),
	}
	err = repo.Save(ctx, msg)
	require.NoError(t, err, "error saving message")

	// Retrieve the message by its ID.
	retrieved, err := repo.SearchByID(ctx, msg.ID)
	require.NoError(t, err, "error retrieving message by ID")
	require.Equal(t, msg.Subject, retrieved.Subject, "retrieved message subject should match saved subject")

	// Query messages based on a text search.
	results, total, took, err := repo.Search(ctx, "testingstream", nil, nil, "", "", 1, 10, "", "")
	require.NoError(t, err, "error searching messages")
	require.GreaterOrEqual(t, total, int64(1), "should find at least one matching message")
	require.Greater(t, took, int64(0), "took value should be positive")
	require.NotEmpty(t, results, "results should not be empty")

	// Update the message.
	updatedSubject := "updated subject"
	update := messagerepo.MessageUpdate{
		Subject: &updatedSubject,
	}
	err = repo.Update(ctx, msg.ID, update)
	require.NoError(t, err, "error updating message")

	// Verify the update.
	updated, err := repo.SearchByID(ctx, msg.ID)
	require.NoError(t, err, "error retrieving updated message")
	require.Equal(t, "updated subject", updated.Subject, "subject should have been updated")

	// Delete the message.
	err = repo.Delete(ctx, msg.ID)
	require.NoError(t, err, "error deleting message")

	// Attempt to retrieve the deleted message.
	_, err = repo.SearchByID(ctx, msg.ID)
	require.Error(t, err, "expected error retrieving a deleted message")
}
