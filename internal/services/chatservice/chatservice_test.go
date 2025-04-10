package chatservice_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/runtimestate"
	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/messagerepo"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/internal/services/chatservice"
	"github.com/js402/CATE/libs/libbus"
	"github.com/js402/CATE/libs/libdb"
	"github.com/js402/CATE/libs/libollama"
	"github.com/js402/CATE/libs/libroutine"
	"github.com/stretchr/testify/require"
)

func SetupTestEnvironment(t *testing.T) (context.Context, *runtimestate.State, func()) {
	ctx := context.TODO()
	err := serverops.NewServiceManager(&serverops.Config{
		JWTExpiry: "1h",
	})
	require.NoError(t, err)
	// We'll collect cleanup functions as we go.
	var cleanups []func()
	addCleanup := func(fn func()) {
		cleanups = append(cleanups, fn)
	}

	// Start local Ollama instance.
	ollamaURI, _, ollamaCleanup, err := libollama.SetupLocalInstance(ctx)
	if err != nil {
		t.Fatalf("failed to start local Ollama instance: %v", err)
	}
	addCleanup(ollamaCleanup)

	// Initialize test database.
	dbConn, _, dbCleanup, err := libdb.SetupLocalInstance(ctx, uuid.NewString(), "test", "test")
	if err != nil {
		for _, fn := range cleanups {
			fn()
		}
		t.Fatalf("failed to setup local database: %v", err)
	}
	addCleanup(dbCleanup)

	dbInstance, err := libdb.NewPostgresDBManager(ctx, dbConn, store.Schema)
	if err != nil {
		for _, fn := range cleanups {
			fn()
		}
		t.Fatalf("failed to create new Postgres DB Manager: %v", err)
	}
	ps, cleanup2 := libbus.NewTestPubSub(t)
	addCleanup(cleanup2)

	// Initialize backend service state.
	backendState, err := runtimestate.New(ctx, dbInstance, ps)
	if err != nil {
		for _, fn := range cleanups {
			fn()
		}
		t.Fatalf("failed to create new backend state: %v", err)
	}

	triggerChan := make(chan struct{})
	// Use the circuit breaker loop to run the state service cycles.
	breaker := libroutine.NewRoutine(3, 1*time.Second)
	go breaker.Loop(ctx, time.Second, triggerChan, backendState.RunBackendCycle, func(err error) {})
	breaker2 := libroutine.NewRoutine(3, 1*time.Second)
	go breaker2.Loop(ctx, time.Second, triggerChan, backendState.RunDownloadCycle, func(err error) {})
	// Register cleanup for the trigger channel.
	addCleanup(func() { close(triggerChan) })

	// Create backend and append model.
	dbStore := store.New(dbInstance.WithoutTransaction())
	backendID := uuid.NewString()
	err = dbStore.CreateBackend(ctx, &store.Backend{
		ID:      backendID,
		Name:    "test-backend",
		BaseURL: ollamaURI,
		Type:    "Ollama",
	})
	if err != nil {
		for _, fn := range cleanups {
			fn()
		}
		t.Fatalf("failed to create backend: %v", err)
	}

	// Append model to the global model store.
	err = dbStore.AppendModel(ctx, &store.Model{
		Model: "smollm2:135m",
	})
	if err != nil {
		for _, fn := range cleanups {
			fn()
		}
		t.Fatalf("failed to append model: %v", err)
	}

	// Trigger sync and wait for model pull.
	triggerChan <- struct{}{}
	require.Eventually(t, func() bool {
		currentState := backendState.Get(ctx)
		r, err := json.Marshal(currentState)
		if err != nil {
			t.Logf("error marshaling state: %v", err)
			return false
		}
		dst := &bytes.Buffer{}
		if err := json.Compact(dst, r); err != nil {
			t.Logf("error compacting JSON: %v", err)
			return false
		}
		return strings.Contains(string(r), `"pulledModels":[{"name":"smollm2:135m"`)
	}, 2*time.Minute, 100*time.Millisecond)

	// Return a cleanup function that calls all cleanup functions.
	cleanupAll := func() {
		for _, fn := range cleanups {
			fn()
		}
	}
	return ctx, backendState, cleanupAll
}

func TestChat(t *testing.T) {
	ctx, backendState, cleanup := SetupTestEnvironment(t)
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
