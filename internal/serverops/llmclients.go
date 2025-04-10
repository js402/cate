package serverops

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Client interfaces for different capabilities
type LLMChatClient interface {
	Chat(ctx context.Context, Messages []Message) (Message, error)
}

type LLMEmbedClient interface {
	Embed(ctx context.Context, prompt string) ([]float32, error)
}

type LLMStreamClient interface {
	Stream(ctx context.Context, prompt string) (<-chan string, error)
}
