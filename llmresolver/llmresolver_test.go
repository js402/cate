package llmresolver_test

import (
	"context"
	"errors"
	"testing"

	"github.com/js402/cate/llmresolver"
	"github.com/js402/cate/modelprovider"
)

func TestResolveCommon(t *testing.T) {
	tests := []struct {
		name        string
		req         llmresolver.ResolveRequest
		providers   []modelprovider.Provider
		wantErr     error
		wantModelID string
	}{
		{
			name: "happy path - exact model match",
			req: llmresolver.ResolveRequest{
				ModelNames:    []string{"llama2:latest"},
				ContextLength: 4096,
			},
			providers: []modelprovider.Provider{
				&modelprovider.MockProvider{
					ID:            "1",
					Name:          "llama2:latest",
					ContextLength: 4096,
					CanChatFlag:   true,
					Backends:      []string{"b1"},
				},
			},
			wantModelID: "1",
		},
		{
			name:      "no models available",
			req:       llmresolver.ResolveRequest{},
			providers: []modelprovider.Provider{},
			wantErr:   llmresolver.ErrNoAvailableModels,
		},
		{
			name: "insufficient context length",
			req: llmresolver.ResolveRequest{
				ContextLength: 8000,
			},
			providers: []modelprovider.Provider{
				&modelprovider.MockProvider{
					ContextLength: 4096,
					CanChatFlag:   true,
				},
			},
			wantErr: llmresolver.ErrNoSatisfactoryModel,
		},
		{
			name: "model exists but name mismatch",
			req: llmresolver.ResolveRequest{
				ModelNames: []string{"smollm2:135m"},
			},
			providers: []modelprovider.Provider{
				&modelprovider.MockProvider{
					ID:            "2",
					Name:          "smollm2",
					ContextLength: 4096,
					CanChatFlag:   true,
					Backends:      []string{"b2"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getModels := func(_ context.Context, _ string) ([]modelprovider.Provider, error) {
				return tt.providers, nil
			}

			_, err := llmresolver.ResolveChat(context.Background(), tt.req, getModels)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}
