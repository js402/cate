package chatservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/js402/CATE/internal/llmresolver"
	"github.com/js402/CATE/internal/modelprovider"
	"github.com/js402/CATE/internal/runtimestate"
	"github.com/js402/CATE/internal/serverops"
	"github.com/js402/CATE/internal/serverops/messagerepo"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/js402/CATE/libs/libollama"
	"github.com/ollama/ollama/api"
)

type Service struct {
	state     *runtimestate.State
	msgRepo   messagerepo.Store
	tokenizer libollama.Tokenizer
}

func New(state *runtimestate.State, msgStore messagerepo.Store, tokenizer libollama.Tokenizer) *Service {
	return &Service{
		state:     state,
		msgRepo:   msgStore,
		tokenizer: tokenizer,
	}
}

type ChatInstance struct {
	Messages []serverops.Message

	CreatedAt time.Time
}

type ChatSession struct {
	ChatID      string       `json:"id"`
	StartedAt   time.Time    `json:"startedAt"`
	BackendID   string       `json:"backendId"`
	LastMessage *ChatMessage `json:"lastMessage,omitempty"`
}

// NewInstance creates a new chat instance after verifying that the user is authorized to start a chat for the given model.
func (s *Service) NewInstance(ctx context.Context, subject string, preferredModels ...string) (uuid.UUID, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return uuid.Nil, err
	}

	// TODO: check if at least one of preferred models are ready for usage.
	// OR we find best candidates instead

	chatSubjectID := uuid.New()
	now := time.Now().UTC()

	err := s.msgRepo.Save(ctx, messagerepo.Message{
		ID:          uuid.New().String(),
		MessageID:   "0",
		Data:        `{"role": "system", "content": "{}"}`,
		Source:      "chatservice",
		SpecVersion: "v1",
		Type:        "chat_message",
		Subject:     chatSubjectID.String(),
		Time:        now,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return chatSubjectID, nil
}

// AddInstruction adds a system instruction to an existing chat instance.
// This method requires admin panel permissions.
func (s *Service) AddInstruction(ctx context.Context, id uuid.UUID, message string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	err := s.msgRepo.Save(ctx, messagerepo.Message{
		ID:          uuid.New().String(),
		MessageID:   "0",
		Data:        fmt.Sprintf(`{"role": "system", "content": "%s"}`, message),
		Source:      "chatservice",
		SpecVersion: "v1",
		Type:        "chat_message",
		Subject:     id.String(),
		Time:        time.Now().UTC(),
	})
	return err
}

func (s *Service) AddMessage(ctx context.Context, id uuid.UUID, message string) error {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionManage); err != nil {
		return err
	}
	err := s.msgRepo.Save(ctx, messagerepo.Message{
		ID:          uuid.New().String(),
		MessageID:   "0",
		Data:        fmt.Sprintf(`{"role": "user", "content": "%s"}`, message),
		Source:      "chatservice",
		SpecVersion: "v1",
		Type:        "chat_message",
		Subject:     id.String(),
		Time:        time.Now().UTC(),
	})
	return err
}

func (s *Service) Chat(ctx context.Context, subjectID uuid.UUID, message string, preferredModelNames ...string) (string, error) {
	// Save the user's message.
	if err := s.AddMessage(ctx, subjectID, message); err != nil {
		return "", err
	}

	// Retrieve all messages for this chat from the persistent store.
	msgs, _, _, err := s.msgRepo.Search(ctx, subjectID.String(), nil, nil, "", "", 0, 10000, "", "")
	if err != nil {
		return "", err
	}

	// Convert stored messages into the api.Message slice.
	var messages []serverops.Message
	for _, msg := range msgs {
		var parsedMsg serverops.Message
		if err := json.Unmarshal([]byte(msg.Data), &parsedMsg); err != nil {
			fmt.Printf("BUG: TODO: json.Unmarshal([]byte(msg.Data): now what? %v", err)
			continue
		}
		messages = append(messages, parsedMsg)
	}

	convertedMessage := make([]api.Message, len(messages))
	for _, m := range messages {
		convertedMessage = append(convertedMessage, api.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	contextLength, err := s.CalculateContextSize(messages)
	if err != nil {
		return "", fmt.Errorf("could not estimate context size %w", err)
	}
	chatClient, err := llmresolver.ResolveChat(ctx, llmresolver.ResolveRequest{
		ContextLength: contextLength,
		ModelNames:    preferredModelNames,
	}, modelprovider.ModelProviderAdapter(ctx, s.state.Get(ctx)))
	if err != nil {
		return "", fmt.Errorf("failed to resolve backend %w", err)
	}
	responseMessage, err := chatClient.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("failed to chat %w", err)
	}
	err = s.msgRepo.Save(ctx, messagerepo.Message{
		ID:          uuid.New().String(),
		MessageID:   "0",
		Data:        fmt.Sprintf(`{"role": "%s", "content": "%s"}`, responseMessage.Role, responseMessage.Content),
		Source:      "chatservice",
		SpecVersion: "v1",
		Type:        "chat_message",
		Subject:     subjectID.String(),
		Time:        time.Now().UTC(),
	})
	if err != nil {
		return "", err
	}

	return responseMessage.Content, nil
}

// ChatMessage is the public representation of a message in a chat.
type ChatMessage struct {
	Role     string    `json:"role"`     // user/assistant/system
	Content  string    `json:"content"`  // message text
	SentAt   time.Time `json:"sentAt"`   // timestamp
	IsUser   bool      `json:"isUser"`   // derived from role
	IsLatest bool      `json:"isLatest"` // mark if last message
}

// GetChatHistory retrieves the chat history for a specific chat instance.
// It checks that the caller is authorized to view the chat instance.
func (s *Service) GetChatHistory(ctx context.Context, id uuid.UUID) ([]ChatMessage, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}

	msgs, _, _, err := s.msgRepo.Search(ctx, id.String(), nil, nil, "", "", 0, 10000, "", "")
	if err != nil {
		return nil, err
	}

	var history []ChatMessage
	for _, msg := range msgs {
		var parsedMsg api.Message
		if err := json.Unmarshal([]byte(msg.Data), &parsedMsg); err != nil {
			continue // Skip messages that cannot be parsed.
		}
		history = append(history, ChatMessage{
			Role:    parsedMsg.Role,
			Content: parsedMsg.Content,
			SentAt:  msg.Time,
			IsUser:  parsedMsg.Role == "user",
		})
	}
	if len(history) > 0 {
		history[len(history)-1].IsLatest = true
	}
	return history, nil
}

// ListChats returns all chat sessions.
// This operation requires admin panel view permission.
func (s *Service) ListChats(ctx context.Context) ([]ChatSession, error) {
	if err := serverops.CheckServiceAuthorization(ctx, s, store.PermissionView); err != nil {
		return nil, err
	}

	// Retrieve messages related to chat sessions.
	msgs, _, _, err := s.msgRepo.Search(ctx, "", nil, nil, "chatservice", "chat_message", 0, 10000, "", "")
	if err != nil {
		return nil, err
	}

	// Group messages by their Subject (chat session id).
	sessionsMap := make(map[string][]messagerepo.Message)
	for _, msg := range msgs {
		sessionsMap[msg.Subject] = append(sessionsMap[msg.Subject], msg)
	}

	var sessions []ChatSession
	for subject, messages := range sessionsMap {
		// Sort messages by time.
		sort.Slice(messages, func(i, j int) bool {
			return messages[i].Time.Before(messages[j].Time)
		})
		// TODO Retrieve a model value.
		var lastMsg *ChatMessage
		if len(messages) > 0 {
			last := messages[len(messages)-1]
			var parsedMsg api.Message
			if err := json.Unmarshal([]byte(last.Data), &parsedMsg); err == nil {
				lastMsg = &ChatMessage{
					Role:     parsedMsg.Role,
					Content:  parsedMsg.Content,
					SentAt:   last.Time,
					IsUser:   parsedMsg.Role == "user",
					IsLatest: true,
				}
			}
		}

		sessions = append(sessions, ChatSession{
			ChatID:      subject,
			StartedAt:   messages[0].Time,
			LastMessage: lastMsg,
		})
	}

	return sessions, nil
}

type ModelResult struct {
	Model      string
	TokenCount int
	MaxTokens  int // Max token length for the model.
}

func (s *Service) CalculateContextSize(messages []serverops.Message, baseModels ...string) (int, error) {
	var prompt string
	for _, m := range messages {
		if m.Role == "user" {
			prompt = prompt + "\n" + m.Content
		}
	}
	var selectedModel string
	for _, model := range baseModels {
		optimal, err := s.tokenizer.OptimalTokenizerModel(model)
		if err != nil {
			return 0, fmt.Errorf("BUG: failed to get optimal model for %q: %w", model, err)
		}
		// TODO: For now, pick the first valid one.
		selectedModel = optimal
		break
	}
	// If no base models were provided, use a fallback.
	if selectedModel == "" {
		selectedModel = "tiny"
	}

	count, err := s.tokenizer.CountTokens(selectedModel, prompt)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate context size %w", err)
	}
	return count, nil
}

func (s *Service) GetServiceName() string {
	return "chatservice"
}

func (s *Service) GetServiceGroup() string {
	return serverops.DefaultDefaultServiceGroup
}
