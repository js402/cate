package chatservice_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/serverops/messagerepo"
	"github.com/js402/CATE/internal/services/chatservice"
	"github.com/js402/CATE/libs/libollama"
	"github.com/stretchr/testify/require"
)

func TestChat(t *testing.T) {
	ctx, backendState, cleanup := chatservice.SetupTestEnvironment(t)
	defer cleanup()
	repo, cleanup2, err := messagerepo.NewTestStore(t)
	require.NoError(t, err, "failed to initialize test repository")
	defer cleanup2()
	tokenizer, err := libollama.NewTokenizer(libollama.TokenizerWithFallbackModel("tiny"), libollama.TokenizerWithPreloadedModels("tiny"))
	if err != nil {
		t.Fatal(err)
	}
	t.Run("creating new chat instance", func(t *testing.T) {
		manager := chatservice.New(backendState, repo, tokenizer)

		// Test valid model
		id, err := manager.NewInstance(ctx, "user1", "smollm2:135m")
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, id)

		// // Test invalid model
		// _, err = manager.NewInstance(ctx, "user1", "invalid-model")
		// require.ErrorContains(t, err, "not ready for usage")
	})

	t.Run("simple chat interaction tests", func(t *testing.T) {
		manager := chatservice.New(backendState, repo, tokenizer)

		id, err := manager.NewInstance(ctx, "user1", "smollm2:135m")
		require.NoError(t, err)
		response, err := manager.Chat(ctx, id, "what is the capital of england?", "smollm2:135m")
		require.NoError(t, err)
		responseLower := strings.ToLower(response)
		println(responseLower)
		require.Contains(t, responseLower, "london")
	})

	t.Run("test chat history via interactions", func(t *testing.T) {
		manager := chatservice.New(backendState, repo, tokenizer)

		// Create new chat instance
		id, err := manager.NewInstance(ctx, "user1", "smollm2:135m")
		require.NoError(t, err)

		// Verify initial empty history
		history, err := manager.GetChatHistory(ctx, id)
		require.NoError(t, err)
		require.Len(t, history, 1, "new instance should have empty history")

		// First interaction
		userMessage1 := "What's the capital of France?"
		_, err = manager.Chat(ctx, id, userMessage1, "smollm2:135m")
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
		// Verify first pair of messages
		history, err = manager.GetChatHistory(ctx, id)
		require.NoError(t, err)
		require.Len(t, history, 3, "should have user + assistant messages")

		// Check user message details
		userMsg := history[1]
		require.Equal(t, "user", userMsg.Role)
		require.Equal(t, userMessage1, userMsg.Content)
		require.True(t, userMsg.IsUser)
		require.False(t, userMsg.IsLatest)
		require.False(t, userMsg.SentAt.IsZero())

		// Check assistant message details
		assistantMsg := history[2]
		require.Equal(t, "assistant", assistantMsg.Role)
		require.NotEmpty(t, assistantMsg.Content)
		require.False(t, assistantMsg.IsUser)
		require.True(t, assistantMsg.IsLatest)
		require.True(t, assistantMsg.SentAt.After(userMsg.SentAt))

		// Second interaction
		userMessage2 := "What about Germany?"
		_, err = manager.Chat(ctx, id, userMessage2, "smollm2:135m")
		require.NoError(t, err)

		// Verify updated history
		history, err = manager.GetChatHistory(ctx, id)
		require.NoError(t, err)
		require.Len(t, history, 5, "should accumulate messages")

		// Verify message order and flags
		secondUserMsg := history[3]
		require.Equal(t, userMessage2, secondUserMsg.Content)
		require.True(t, secondUserMsg.SentAt.After(assistantMsg.SentAt))

		finalAssistantMsg := history[4]
		require.True(t, finalAssistantMsg.IsLatest)
		require.Contains(t, strings.ToLower(finalAssistantMsg.Content), "germany", "should maintain context")

		// Verify all timestamps are sequential
		for i := 1; i < len(history); i++ {
			require.True(t, history[i].SentAt.After(history[i-1].SentAt),
				"message %d should be after message %d", i, i-1)
		}

		// Test invalid ID case
		hist, err := manager.GetChatHistory(ctx, uuid.New())
		require.NoError(t, err)
		require.Len(t, hist, 0)
	})
}
