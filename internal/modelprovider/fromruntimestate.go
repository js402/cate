package modelprovider

import (
	"context"

	"github.com/js402/CATE/internal/runtimestate"
)

// RuntimeState retrieves available model providers for a specific backend type
type RuntimeState func(ctx context.Context, backendType string) ([]Provider, error)

func ModelProviderAdapter(ctx context.Context, runtime map[string]runtimestate.LLMState) RuntimeState {
	models := make(map[string][]string)
	for _, state := range runtime {
		for _, model := range state.PulledModels {
			models[model.Model] = append(models[model.Model], state.Backend.BaseURL)
		}
	}
	res := []Provider{}
	for model, backends := range models {
		provider := NewOllamaModelProvider(model, backends, WithChat(true),
			WithContextLength(8192),
		)
		res = append(res, provider)
	}
	return func(ctx context.Context, backendType string) ([]Provider, error) {
		var providers []Provider
		for _, provider := range res {
			// if provider.BackendType() == backendType { // TODO: Implement backend type filtering
			providers = append(providers, provider)
			// }
		}
		return providers, nil
	}
}
