package libollama

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"maps"

	"github.com/ollama/ollama/llama"
)

// maximum prompt size in bytes to prevent potential segfaults
// observed with very large inputs (e.g., >16KB) in the underlying library.
const maxPromptBytes = 16 * 1024 // 16 KiB

// Tokenizer represents an interface for tokenizing text using a specific model.
type Tokenizer interface {
	// CountTokens counts the number of tokens in the given prompt using the specified model.
	// When to Use:
	// - When you need the token count but not the actual tokens.
	// - For validating prompt length against model limits (e.g., before API calls).
	CountTokens(modelName, prompt string) (int, error)
	// Tokenize tokenizes the given prompt using the specified model.
	Tokenize(modelName, prompt string) ([]int, error)
	// AvailableModels returns a list of available models that can be used for tokenization.
	// This method is useful when you need to know which models are available for tokenization.
	AvailableModels() []string
	// OptimalTokenizerModel returns the optimal model for tokenization based on the given model.
	// This is useful when the basedOnModel is not available in the list of available models.
	// The implementation is based on the tokenizer model mappings.
	// Logic flow:
	// - Checks for exact matches in configured models.
	// - Falls back to substring matches (e.g., phi3 → phi-3).
	// - Uses a fallback model (default: llama-3.1) if no match is found.
	OptimalTokenizerModel(basedOnModel string) (string, error)
}

// TokenizerModelMappings represents
// a mapping between a canonical model name and its substrings.
type TokenizerModelMappings struct {
	CanonicalName string
	Substrings    []string
}

// NewTokenizer creates a new tokenizer instance with the specified options.
// This implementation uses the tokenizer model mappings to determine the optimal model.
// It does not determine the optimal model dynamically instead it uses the default and/or provided mappings.
// Additional options can be provided if needed. All models will be loaded from the default model URLs, if not found on disk.
// the default download path is "~/.libollama" ensure the Host has enough disk space.
// tokenisation is done using the downloaded .gguf format models via the ollama/ollama/llama tokenizer.
// When a model is already used once it will be cached in memory.
// Concurrency: Methods are thread-safe, but model downloads block other operations.
// Performance: Tokenization is fast, but model loading incurs initial latency (mitigated by preloading).
// REMEMBER: The performance of tokenization is highly dependent on the model phi would be able to have 60.39 MB/s while tiny only 11.10 MB/s.
//
// Example usage:
//
//	// Initialize with a custom model and preload
//	tokenizer, _ := libollama.NewTokenizer(
//		libollama.TokenizerWithCustomModels(map[string]string{"custom": "https://example.com/custom.gguf"}),
//		libollama.TokenizerWithPreloadedModels("llama-3.1"),
//	)
//	// Determine the optimal model for a user-provided name
//	model, _ := tokenizer.OptimalTokenizerModel("llama3-14b")
//	fmt.Printf("Using model: %s\n", model) // Output: Using model: llama-3.1
//
// tokens, _ := tokenizer.Tokenize(model, "Hello, world!")
// fmt.Printf("Tokens: %v\n", tokens)
func NewTokenizer(opts ...TokenizerOption) (Tokenizer, error) {
	defaultModelURLs := map[string]string{
		"tiny":      "https://huggingface.co/Hjgugugjhuhjggg/FastThink-0.5B-Tiny-Q2_K-GGUF/resolve/main/fastthink-0.5b-tiny-q2_k.gguf",
		"llama-3.1": "https://huggingface.co/bartowski/Meta-Llama-3.1-8B-Instruct-GGUF/resolve/main/Meta-Llama-3.1-8B-Instruct-IQ2_M.gguf",
		"llama-3.2": "https://huggingface.co/unsloth/Llama-3.2-3B-Instruct-GGUF/blob/main/Llama-3.2-3B-Instruct-Q2_K.gguf",
		// RESTRICTED: "gemma-2b":  "https://huggingface.co/google/gemma-2b-GGUF/resolve/main/gemma-2b.gguf",
		"phi-3": "https://huggingface.co/microsoft/Phi-3-mini-4k-instruct-gguf/resolve/main/Phi-3-mini-4k-instruct-q4.gguf",
	}

	fallback := "llama-3.1"
	// Heuristic Mapping: Define families and their canonical representatives.
	familyMappings := []TokenizerModelMappings{
		{CanonicalName: "llama-3.2", Substrings: []string{"llama-3.2", "llama3.2"}},
		{CanonicalName: "llama-3.1", Substrings: []string{"llama-3.1", "llama3.1", "llama-3", "llama3"}},
		// RESTRICTED: {CanonicalName: "gemma-2b", Substrings: []string{"gemma", "gemma-2b"}},
		{CanonicalName: "phi-3", Substrings: []string{"phi-3", "phi3"}},
	}

	rt := &ollamatokenizer{
		modelURLs:      defaultModelURLs,
		loadedModels:   make(map[string]*llama.Model),
		httpClient:     http.DefaultClient,
		mu:             sync.RWMutex{},
		fallback:       fallback,
		familyMappings: familyMappings,
		token:          "",
	}

	for _, opt := range opts {
		if err := opt(rt); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

type ollamatokenizer struct {
	modelURLs      map[string]string
	loadedModels   map[string]*llama.Model
	mu             sync.RWMutex
	familyMappings []TokenizerModelMappings
	fallback       string
	httpClient     *http.Client
	token          string
}

// AvailableModels implements Tokenizer.
func (c *ollamatokenizer) AvailableModels() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	models := make([]string, 0, len(c.modelURLs))
	for model := range c.modelURLs {
		models = append(models, model)
	}
	return models
}

type TokenizerOption func(*ollamatokenizer) error

// Add or override model URLs without replacing the defaults.
// This allows to expand or update the model URLs.
func TokenizerWithCustomModels(models map[string]string) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		maps.Copy(rt.modelURLs, models)
		return nil
	}
}

// TokenizerWithModelMap Replaces the default model URLs entirely.
func TokenizerWithModelMap(models map[string]string) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		rt.modelURLs = models
		return nil
	}
}

// TokenizerWithFallbackModel Changes the fallback model (default: llama-3.1).
func TokenizerWithFallbackModel(model string) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		rt.fallback = model
		return nil
	}
}

// TokenizerWithPreloadedModels Downloads the model and preloads models into memory.
// Use this to make the first tokenizer usage more responsive.
// Or to ensure the models are downloaded without errors.
func TokenizerWithPreloadedModels(models ...string) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		for _, m := range models {
			if _, err := rt.loadModel(m); err != nil {
				return fmt.Errorf("failed to preload model %s: %w", m, err)
			}
		}
		return nil
	}
}

// Use a custom HTTP client (e.g., for proxies or timeouts).
func TokenizerWithHTTPClient(client *http.Client) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		rt.httpClient = client
		return nil
	}
}

// TokenizerWithToken sets the API token for downloading restricted models.
// The token will be used in the Authorization header for requests to huggingface.co.
// Useful for huggingface. Get your token from https://huggingface.co/settings/tokens
func TokenizerWithToken(token string) TokenizerOption {
	return func(rt *ollamatokenizer) error {
		rt.mu.Lock()
		defer rt.mu.Unlock()
		rt.token = token
		return nil
	}
}

// getModelURL resolves a model name to a download URL.
func (c *ollamatokenizer) getModelURL(modelName string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	url, ok := c.modelURLs[modelName]
	if !ok {
		return "", fmt.Errorf("unknown model: %s", modelName)
	}
	return url, nil
}

// downloadFile downloads a file from the given URL and writes it to destPath.
func (c *ollamatokenizer) downloadFile(urlStr, destPath string) error {
	fmt.Printf("Attempting to download %s to %s\n", urlStr, destPath)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return fmt.Errorf("failed to create request for %s: %w", urlStr, err)
	}

	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	// sanity check
	_, err = url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("Could not parse URL %s: %w", urlStr, err)
	}

	// Add Authorization header only if token is present
	if token != "" {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	// Use the configured HTTP client to perform the request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed http request to %s: %w", urlStr, err)
	}
	defer resp.Body.Close()

	fmt.Printf("HTTP Status: %s\n", resp.Status)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		errMsg := fmt.Sprintf("bad HTTP status: %s. Body glimpse: %s", resp.Status, string(bodyBytes))
		// Add a hint if auth might be needed
		if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && token == "" {
			errMsg += " (Hint: Does this model require authentication?)"
		}
		return fmt.Errorf("%s", errMsg)
	}

	// Create the destination file *after* successful status check
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", destPath, err)
	}
	defer out.Close() // Ensure file is closed

	bytesWritten, err := io.Copy(out, resp.Body)
	fmt.Printf("Bytes written: %d\n", bytesWritten)
	if err != nil {
		os.Remove(destPath)
		return fmt.Errorf("failed to write file %s after %d bytes: %w", destPath, bytesWritten, err)
	}

	// Sync contents to disk
	if err = out.Sync(); err != nil {
		// Log sync errors but don't necessarily fail the download, consistent with original code
		fmt.Printf("Warning: failed to sync file %s: %v\n", destPath, err)
	}

	fmt.Printf("Successfully downloaded %s\n", destPath)
	return nil
}

// downloadModel downloads the model if it doesn't already exist and returns the path.
func (c *ollamatokenizer) downloadModel(modelName string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	dir := filepath.Join(homeDir, ".libollama", "models", modelName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	url, err := c.getModelURL(modelName)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(dir, "model.gguf")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		if err := c.downloadFile(url, destPath); err != nil {
			return "", err
		}
	}

	return destPath, nil
}

// loadModel loads a model from disk, caching the loaded model in memory.
// This function is safe for concurrent use.
func (c *ollamatokenizer) loadModel(modelName string) (*llama.Model, error) {
	c.mu.RLock()
	if model, exists := c.loadedModels[modelName]; exists {
		c.mu.RUnlock()
		return model, nil
	}
	c.mu.RUnlock()

	// Download the model if necessary.
	modelPath, err := c.downloadModel(modelName)
	if err != nil {
		return nil, fmt.Errorf("failed to download model %s: %w", modelName, err)
	}

	params := llama.ModelParams{
		VocabOnly: true,
		Progress: func(f float32) {
			fmt.Printf("Loading model %s: %.2f%%\n", modelName, f*100)
		},
	}

	model, err := llama.LoadModelFromFile(modelPath, params)
	if err != nil {
		return nil, fmt.Errorf("failed to load model %s from %s: %w", modelName, modelPath, err)
	}

	// Acquire write lock to update the cache.
	c.mu.Lock()
	c.loadedModels[modelName] = model
	c.mu.Unlock()

	fmt.Printf("Successfully loaded model %s\n", modelName)
	return model, nil
}

func (c *ollamatokenizer) CountTokens(modelName, prompt string) (int, error) {
	a, err := c.Tokenize(modelName, prompt)
	if err != nil {
		return 0, err
	}
	count := len(a)
	return count, nil
}

// Tokenize tokenizes the given text using the specified model.
func (c *ollamatokenizer) Tokenize(modelName, prompt string) ([]int, error) {
	promptLen := len(prompt)
	if promptLen > maxPromptBytes {
		return []int{}, fmt.Errorf("input prompt size (%d bytes) exceeds maximum allowed size (%d bytes)", promptLen, maxPromptBytes)
	}
	model, err := c.loadModel(modelName)
	if err != nil {
		return nil, err
	}
	// TODO: true, true why do we need these parameters?
	tokens, err := model.Tokenize(prompt, true, true)
	if err != nil {
		return nil, fmt.Errorf("tokenization failed: %w", err)
	}

	return tokens, nil
}

func (c *ollamatokenizer) OptimalTokenizerModel(basedOnModel string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.modelURLs == nil || len(c.modelURLs) == 0 {
		return "", fmt.Errorf("No models configured.")
	}
	basedOnModel = strings.ToLower(basedOnModel)
	basedOnModel = strings.Split(basedOnModel, ":")[0]
	if _, exists := c.modelURLs[basedOnModel]; exists {
		return basedOnModel, nil
	}

	for _, mapping := range c.familyMappings {
		if _, canonicalExists := c.modelURLs[mapping.CanonicalName]; !canonicalExists {
			continue
		}

		// Check if the input model name contains any of the identifying substrings
		for _, sub := range mapping.Substrings {
			if strings.Contains(basedOnModel, sub) {
				return mapping.CanonicalName, nil // Found a match, return the canonical representative's name
			}
		}
	}

	return c.fallback, nil
}
