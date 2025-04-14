package modelprovider_test

import (
	"testing"

	"github.com/js402/cate/modelprovider"
	"github.com/js402/cate/serverops"
	"github.com/js402/cate/services/chatservice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaChatClient_HappyPath(t *testing.T) {
	ctx, backendState, cleanup := chatservice.SetupTestEnvironment(t)
	defer cleanup()
	runtime := backendState.Get(ctx)
	url := ""
	for _, state := range runtime {
		url = state.Backend.BaseURL
	}
	require.NotEmpty(t, url, "Failed to get backend URL from test setup") // Added check

	provider := modelprovider.NewOllamaModelProvider("smollm2:135m", []string{url}, modelprovider.WithChat(true))
	require.True(t, provider.CanChat())

	client, err := provider.GetChatConnection(url)
	require.NoError(t, err)
	require.NotNil(t, client)

	// First chat call
	response, err := client.Chat(ctx, []serverops.Message{
		{Content: "Hello, world!", Role: "user"},
	})
	require.NoError(t, err)
	t.Logf("Response 1: %s", response.Content)
	require.NotEmpty(t, response.Content)
	require.Equal(t, "assistant", response.Role)
	response2, err := client.Chat(ctx, []serverops.Message{
		{Content: "Hello, world!", Role: "user"},
		response,
		{Content: "How are you?", Role: "user"},
	})
	require.NoError(t, err)
	t.Logf("Response 2: %s", response2.Content)
	require.NotEmpty(t, response2.Content)
	require.Equal(t, "assistant", response2.Role)
}

func TestOllamaProvider_GetChatConnection_ChatDisabled(t *testing.T) {
	dummyURL := "http://localhost:11434"
	backends := []string{dummyURL}
	modelName := "smollm2:135m"

	provider := modelprovider.NewOllamaModelProvider(modelName, backends, modelprovider.WithChat(false))
	require.False(t, provider.CanChat(), "Provider should report CanChat as false")
	client, err := provider.GetChatConnection(dummyURL)
	require.Error(t, err, "Expected an error when getting chat connection for a non-chat provider")
	assert.Nil(t, client, "Client should be nil on error")
	assert.ErrorContains(t, err, "does not support chat", "Error message should indicate lack of chat support")
}

func TestOllamaChatClient_ChatWithNonExistentModel(t *testing.T) {
	ctx, backendState, cleanup := chatservice.SetupTestEnvironment(t)
	defer cleanup()
	runtime := backendState.Get(ctx)
	url := ""
	for _, state := range runtime {
		url = state.Backend.BaseURL
	}
	require.NotEmpty(t, url, "Failed to get backend URL from test setup")

	nonExistentModel := "this-model-definitely-does-not-exist-12345:latest"
	provider := modelprovider.NewOllamaModelProvider(nonExistentModel, []string{url}, modelprovider.WithChat(true))
	require.True(t, provider.CanChat())

	client, err := provider.GetChatConnection(url)
	require.NoError(t, err, "Getting the client should succeed even if model doesn't exist yet")
	require.NotNil(t, client)

	_, err = client.Chat(ctx, []serverops.Message{
		{Content: "Does not matter", Role: "user"},
	})

	require.Error(t, err, "Expected an error when chatting with a non-existent model")
	assert.ErrorContains(t, err, "ollama API chat request failed", "Error message should indicate API failure")
	assert.ErrorContains(t, err, nonExistentModel, "Error message should mention the problematic model name")
	t.Logf("Confirmed error for non-existent model: %v", err)
}
