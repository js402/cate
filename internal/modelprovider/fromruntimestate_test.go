package modelprovider_test

import (
	"context"
	"testing"
	"time"

	"github.com/js402/CATE/internal/modelprovider"
	"github.com/js402/CATE/internal/runtimestate"
	"github.com/js402/CATE/internal/serverops/store"
	"github.com/ollama/ollama/api"
)

func TestModelProviderAdapter_ReturnsCorrectProviders(t *testing.T) {
	now := time.Now()

	runtime := map[string]runtimestate.LLMState{
		"backend1": {
			ID:      "backend1",
			Name:    "Backend One",
			Backend: store.Backend{ID: "backend1", Name: "Ollama", Type: "ollama"},
			PulledModels: []api.ListModelResponse{
				{Name: "Model One", Model: "model1", ModifiedAt: now},
				{Name: "Model Shared", Model: "shared", ModifiedAt: now},
			},
		},
		"backend2": {
			ID:      "backend2",
			Name:    "Backend Two",
			Backend: store.Backend{ID: "backend2", Name: "Ollama", Type: "ollama"},
			PulledModels: []api.ListModelResponse{
				{Name: "Model Two", Model: "model2", ModifiedAt: now},
				{Name: "Model Shared", Model: "shared", ModifiedAt: now},
			},
		},
	}

	adapter := modelprovider.ModelProviderAdapter(context.Background(), runtime)

	providers, err := adapter(context.Background(), "ollama")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(providers) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(providers))
	}

	// Optional: you can check the actual model names returned
	models := map[string]bool{}
	for _, provider := range providers {
		models[provider.ModelName()] = true
	}

	expected := []string{"model1", "model2", "shared"}
	for _, model := range expected {
		if !models[model] {
			t.Errorf("expected model %q to be in providers, but it was not found", model)
		}
	}
}
