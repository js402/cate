package modelprovider_test

import (
	"fmt"
	"testing"

	"github.com/js402/cate/core/modelprovider"
	"github.com/js402/cate/core/serverops"
	"github.com/js402/cate/core/services/chatservice"
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
	require.NotEmpty(t, url, "Failed to get backend URL from test setup")
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

func TestOllamaChatClient_LongerHappyPath(t *testing.T) {
	ctx, backendState, cleanup := chatservice.SetupTestEnvironment(t)
	defer cleanup()
	runtime := backendState.Get(ctx)
	url := ""
	for _, state := range runtime {
		url = state.Backend.BaseURL
	}
	require.NotEmpty(t, url, "Failed to get backend URL from test setup")

	provider := modelprovider.NewOllamaModelProvider("smollm2:135m", []string{url}, modelprovider.WithChat(true))
	require.True(t, provider.CanChat())

	client, err := provider.GetChatConnection(url)
	require.NoError(t, err)
	require.NotNil(t, client)
	userMessages := []serverops.Message{
		serverops.Message{Content: "Hello, world!", Role: "user"},
		serverops.Message{Content: "How are you?", Role: "user"},
		serverops.Message{Content: "How old are you?", Role: "user"},
		serverops.Message{Content: "Hey", Role: "user"},
		serverops.Message{Content: "Where are you from?", Role: "user"},
		serverops.Message{Content: "What is your favorite color?", Role: "user"},
		serverops.Message{Content: "What is your favorite food?", Role: "user"},
		serverops.Message{Content: "What is your favorite movie?", Role: "user"},
		serverops.Message{Content: "What is your favorite sport?", Role: "user"},
	}
	conversation := func(chat []serverops.Message, prompt string) []serverops.Message {
		chat = append(chat, serverops.Message{Role: "user", Content: prompt})
		response, err := client.Chat(ctx, chat)
		require.NoError(t, err)
		require.NotEmpty(t, response.Content)
		require.Equal(t, "assistant", response.Role)
		fmt.Printf("Response: %s /n", response.Content)

		chat = append(chat, response)
		return chat
	}
	chat := []serverops.Message{}
	for _, message := range userMessages {
		chat = conversation(chat, message.Content)
	}
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
