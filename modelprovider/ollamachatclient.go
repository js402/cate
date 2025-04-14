package modelprovider

import (
	"context"
	"fmt"

	"github.com/js402/cate/serverops"
	"github.com/ollama/ollama/api"
)

type OllamaChatClient struct {
	ollamaClient *api.Client // The underlying Ollama API client
	modelName    string      // The specific model this client targets (e.g., "llama3:latest")
	backendURL   string      // backend URL
}

var _ serverops.LLMChatClient = (*OllamaChatClient)(nil)

func (c *OllamaChatClient) Chat(ctx context.Context, messages []serverops.Message) (serverops.Message, error) {
	apiMessages := make([]api.Message, 0, len(messages))
	for _, msg := range messages {
		apiMessages = append(apiMessages, api.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	stream := false
	req := &api.ChatRequest{
		Model:    c.modelName,
		Messages: apiMessages,
		Stream:   &stream, // Explicitly set stream to false
		// Options: nil, // Add specific Ollama options if needed (e.g., temperature)
	}

	var finalResponse api.ChatResponse

	err := c.ollamaClient.Chat(ctx, req, func(res api.ChatResponse) error {
		// NOTE: This callback should ideally be called only once when stream=false and res.Done=true
		if res.Done {
			finalResponse = res
		} else if res.Message.Content != "" {
			// NOTE: Sometimes non-streaming might still send intermediate message chunks? Capture the last one.
			finalResponse = res
		}
		return nil
	})

	if err != nil {
		return serverops.Message{}, fmt.Errorf("ollama API chat request failed for model %s: %w", c.modelName, err)
	}

	// Ensure finalResponse has meaningful data, especially Message.Content
	if finalResponse.Message.Content == "" && !finalResponse.Done {
		return serverops.Message{}, fmt.Errorf("ollama chat completed without a final message for model %s", c.modelName)
	}

	resultMessage := serverops.Message{
		Role:    finalResponse.Message.Role, // Usually "assistant"
		Content: finalResponse.Message.Content,
	}

	return resultMessage, nil
}
