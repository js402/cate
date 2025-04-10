// Package llmresolver selects the most appropriate backend LLM instance based on requirements.
package llmresolver

import (
	"context"
	"errors"
	"math/rand"
	"strings"

	"github.com/js402/CATE/internal/modelprovider"
	"github.com/js402/CATE/internal/serverops"
)

var (
	ErrNoAvailableModels        = errors.New("no models found in runtime state")
	ErrNoSatisfactoryModel      = errors.New("no model matched the requirements")
	ErrUnknownModelCapabilities = errors.New("capabilities not known for this model")
)

// ResolveRequest contains requirements for selecting a model provider.
type ResolveRequest struct {
	Provider      string   // Optional: if empty, uses default provider
	ModelNames    []string // Optional: if empty, any model is considered
	ContextLength int      // Minimum required context length; 0 means no requirement
}

func resolveCommon(
	ctx context.Context,
	req ResolveRequest,
	getModels modelprovider.RuntimeState,
	capCheck func(modelprovider.Provider) bool,
) (modelprovider.Provider, string, error) {
	providerType := req.Provider
	if providerType == "" {
		providerType = "Ollama" // Default provider
	}

	providers, err := getModels(ctx, providerType)
	if err != nil {
		return nil, "", err
	}
	if len(providers) == 0 {
		return nil, "", ErrNoAvailableModels
	}

	// Use a map to track seen providers by ID to prevent duplicates
	seenProviders := make(map[string]bool)
	var candidates []modelprovider.Provider

	// Handle model name preferences
	if len(req.ModelNames) > 0 {
		// Check preferred models in order of priority
		for _, preferredModel := range req.ModelNames {
			basePreferred := parseModelName(preferredModel)

			for _, p := range providers {
				if seenProviders[p.GetID()] {
					continue
				}

				// Match either base or full name
				currentBase := parseModelName(p.ModelName())
				currentFull := p.ModelName()

				if currentBase != basePreferred && currentFull != preferredModel {
					continue
				}

				if validateProvider(p, req.ContextLength, capCheck) {
					candidates = append(candidates, p)
					seenProviders[p.GetID()] = true
				}
			}
		}
	} else {
		// Consider all providers when no model names specified
		for _, p := range providers {
			if validateProvider(p, req.ContextLength, capCheck) {
				candidates = append(candidates, p)
			}
		}
	}

	if len(candidates) == 0 {
		return nil, "", ErrNoSatisfactoryModel
	}

	// Select random provider from candidates
	selected := candidates[rand.Intn(len(candidates))]

	// Select backend with basic load balancing
	backendIDs := selected.GetBackendIDs()
	if len(backendIDs) == 0 {
		return nil, "", ErrNoAvailableModels
	}
	backendID := backendIDs[rand.Intn(len(backendIDs))]

	return selected, backendID, nil
}

// validateProvider checks if a provider meets requirements
func validateProvider(p modelprovider.Provider, minContext int, capCheck func(modelprovider.Provider) bool) bool {
	if minContext > 0 && p.GetContextLength() < minContext {
		return false
	}
	return capCheck(p)
}

// parseModelName extracts the base model name before the first colon
func parseModelName(modelName string) string {
	if parts := strings.SplitN(modelName, ":", 2); len(parts) > 0 {
		return parts[0]
	}
	return modelName
}

func ResolveChat(
	ctx context.Context,
	req ResolveRequest,
	getModels modelprovider.RuntimeState,
) (serverops.LLMChatClient, error) {
	provider, backend, err := resolveCommon(ctx, req, getModels, modelprovider.Provider.CanChat)
	if err != nil {
		return nil, err
	}
	return provider.GetChatConnection(backend)
}

// ResolveEmbed finds a provider supporting embeddings
func ResolveEmbed(
	ctx context.Context,
	req ResolveRequest,
	getModels modelprovider.RuntimeState,
) (serverops.LLMEmbedClient, error) {
	provider, backend, err := resolveCommon(ctx, req, getModels, modelprovider.Provider.CanEmbed)
	if err != nil {
		return nil, err
	}
	return provider.GetEmbedConnection(backend)
}

// ResolveStream finds a provider supporting streaming
func ResolveStream(
	ctx context.Context,
	req ResolveRequest,
	getModels modelprovider.RuntimeState,
) (serverops.LLMStreamClient, error) {
	provider, backend, err := resolveCommon(ctx, req, getModels, modelprovider.Provider.CanStream)
	if err != nil {
		return nil, err
	}
	return provider.GetStreamConnection(backend)
}
