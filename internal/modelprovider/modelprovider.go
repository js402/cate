package modelprovider

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/js402/CATE/internal/serverops"
	"github.com/ollama/ollama/api"
	_ "github.com/ollama/ollama/api"
)

// Provider is a provider of backend instances capable of executing requests with this Model.
type Provider interface {
	GetBackendIDs() []string // Available backend instances
	ModelName() string       // Model name (e.g., "llama2:latest")
	GetID() string           // unique identifier for the model provider
	GetContextLength() int   // Maximum context length supported
	CanChat() bool           // Supports chat interactions
	CanEmbed() bool          // Supports embeddings
	CanStream() bool         // Supports streaming
	GetChatConnection(backendID string) (serverops.LLMChatClient, error)
	GetEmbedConnection(backendID string) (serverops.LLMEmbedClient, error)
	GetStreamConnection(backendID string) (serverops.LLMStreamClient, error)
}

type OllamaProvider struct {
	Name           string
	ID             string
	ContextLength  int
	SupportsChat   bool
	SupportsEmbed  bool
	SupportsStream bool
	Backends       []string // we assume that Backend IDs are urls to the instance
}

func (p *OllamaProvider) GetBackendIDs() []string {
	return p.Backends
}

func (p *OllamaProvider) ModelName() string {
	return p.Name
}

func (p *OllamaProvider) GetID() string {
	return p.ID
}

func (p *OllamaProvider) GetContextLength() int {
	return p.ContextLength
}

func (p *OllamaProvider) CanChat() bool {
	return p.SupportsChat
}

func (p *OllamaProvider) CanEmbed() bool {
	return p.SupportsEmbed
}

func (p *OllamaProvider) CanStream() bool {
	return p.SupportsStream
}

func (p *OllamaProvider) GetChatConnection(backendID string) (serverops.LLMChatClient, error) {
	if !p.CanChat() {
		return nil, fmt.Errorf("provider %s (model %s) does not support chat", p.GetID(), p.ModelName())
	}
	u, err := url.Parse(backendID)
	if err != nil {
		// Consider logging the error too
		return nil, fmt.Errorf("invalid backend URL '%s' for provider %s: %w", backendID, p.GetID(), err)
	}
	// TODO: Consider using a configurable http.Client with timeouts
	httpClient := http.DefaultClient
	ollamaAPIClient := api.NewClient(u, httpClient)

	// Create and return the wrapper client
	chatClient := &OllamaChatClient{
		ollamaClient: ollamaAPIClient,
		modelName:    p.ModelName(), // Use the full model name (e.g., "llama3:latest")
		backendURL:   backendID,
	}

	return chatClient, nil
}

func (p *OllamaProvider) GetEmbedConnection(backendID string) (serverops.LLMEmbedClient, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (p *OllamaProvider) GetStreamConnection(backendID string) (serverops.LLMStreamClient, error) {
	return nil, fmt.Errorf("unimplemented")
}

type OllamaOption func(*OllamaProvider)

func NewOllamaModelProvider(name string, backends []string, opts ...OllamaOption) Provider {
	// Define defaults based on model name
	context := modelContextLengths[name]
	canChat := canChat[name]
	canEmbed := canEmbed[name]
	canStream := canStreaming[name]

	p := &OllamaProvider{
		Name:           name,
		ID:             "ollama:" + name,
		ContextLength:  context,
		SupportsChat:   canChat,
		SupportsEmbed:  canEmbed,
		SupportsStream: canStream,
		Backends:       backends,
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

var (
	modelContextLengths = map[string]int{
		"llama2":          4096,
		"llama3":          8192,
		"mistral":         8192,  // Mistral 7B
		"mixtral":         32768, // Mixtral 8x7B
		"phi":             2048,  // Phi-2
		"codellama":       16384, // CodeLlama 34B
		"gemma":           8192,  // Gemma 2B/7B
		"openhermes":      4096,  // Based on OpenChat or Mistral
		"notux":           4096,  // Notux (fine-tuned Mistral)
		"llava":           8192,  // LLaVA (multimodal, Mistral-based)
		"deepseek":        8192,  // DeepSeek-Coder
		"qwen":            8192,  // Qwen 7B
		"zephyr":          8192,  // Zephyr 7B (based on Mistral)
		"neural-chat":     8192,  // Intel fine-tuned LLM
		"dolphin-mixtral": 32768, // Mixtral fine-tune
	}

	modelContextLengthsFullNames = map[string]int{
		"codellama:34b-100k":     100000,
		"mixtral-8x7b":           32768,
		"dolphin-mixtral:latest": 32768,
	}

	canChat = map[string]bool{
		"llama2": true, "llama3": true, "mistral": true,
		"mixtral": true, "phi": true, "codellama": true,
		"gemma": true, "openhermes": true, "notux": true,
		"llava": true, "deepseek": true, "qwen": true,
		"zephyr": true, "neural-chat": true, "dolphin-mixtral": true,
	}

	canEmbed = map[string]bool{
		"deepseek": true,
		"qwen":     true,
	}

	canStreaming = map[string]bool{
		"llama2": true, "llama3": true, "mistral": true,
		"mixtral": true, "phi": true, "codellama": true,
		"gemma": true, "openhermes": true, "notux": true,
		"llava": true, "deepseek": true, "qwen": true,
		"zephyr": true, "neural-chat": true, "dolphin-mixtral": true,
	}
)

func WithChat(supports bool) OllamaOption {
	return func(p *OllamaProvider) {
		p.SupportsChat = supports
	}
}

func WithEmbed(supports bool) OllamaOption {
	return func(p *OllamaProvider) {
		p.SupportsEmbed = supports
	}
}

func WithStream(supports bool) OllamaOption {
	return func(p *OllamaProvider) {
		p.SupportsStream = supports
	}
}

func WithContextLength(length int) OllamaOption {
	return func(p *OllamaProvider) {
		p.ContextLength = length
	}
}

func WithComputedContextLength(model ListModelResponse) OllamaOption {
	return func(p *OllamaProvider) {
		length, err := GetModelsMaxContextLength(model)
		if err != nil {
			baseName := parseModelName(model.Model)
			length = modelContextLengths[baseName] // fallback
		}
		p.ContextLength = length
	}
}

// GetModelsMaxContextLength returns the effective max context length with improved handling
func GetModelsMaxContextLength(model ListModelResponse) (int, error) {
	fullModelName := model.Model

	// 1. Check full model name override first
	if ctxLen, ok := modelContextLengthsFullNames[fullModelName]; ok {
		return ctxLen, nil
	}

	// 2. Try base model name
	baseModelName := parseModelName(fullModelName)
	baseCtxLen, ok := modelContextLengths[baseModelName]
	if !ok {
		return 0, fmt.Errorf("base name '%s' from model '%s'", baseModelName, fullModelName)
	}

	// 3. Apply smart adjustments based on model metadata
	adjustedCtxLen := baseCtxLen
	details := model.Details

	// Handle special cases using model metadata
	switch {
	case containsAny(details.Families, []string{"extended-context", "long-context"}):
		adjustedCtxLen = int(float64(adjustedCtxLen) * 2.0)
	case details.ParameterSize == "70B" && baseModelName == "llama3":
		// Llama3 70B uses Grouped Query Attention for better context handling
		adjustedCtxLen = 8192 // Explicit set as official context window
	case strings.Contains(details.QuantizationLevel, "4-bit"):
		// Quantized models might have reduced effective context
		adjustedCtxLen = int(float64(adjustedCtxLen) * 0.8)
	}

	// 4. Cap values based on known limits
	if maxCap, ok := modelContextLengthsFullNames[baseModelName+"-max"]; ok {
		if adjustedCtxLen > maxCap {
			adjustedCtxLen = maxCap
		}
	}

	return adjustedCtxLen, nil
}

// Helper function for slice contains check
func containsAny(slice []string, items []string) bool {
	for _, s := range slice {
		for _, item := range items {
			if strings.EqualFold(s, item) {
				return true
			}
		}
	}
	return false
}

// parseModelName extracts the base model name before the first colon
func parseModelName(modelName string) string {
	if parts := strings.SplitN(modelName, ":", 2); len(parts) > 0 {
		return parts[0]
	}
	return modelName
}

type ListModelResponse struct {
	Name       string       `json:"name"`
	Model      string       `json:"model"`
	ModifiedAt time.Time    `json:"modified_at"`
	Size       int64        `json:"size"`
	Digest     string       `json:"digest"`
	Details    ModelDetails `json:"details"`
}

type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}
