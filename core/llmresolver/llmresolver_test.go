package llmresolver_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/js402/cate/core/llmresolver"
	"github.com/js402/cate/core/modelprovider"
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

			_, err := llmresolver.ResolveChat(context.Background(), tt.req, getModels, llmresolver.ResolveRandomly)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveEmbed(t *testing.T) {
	// Define common providers used in tests
	providerEmbedOK := &modelprovider.MockProvider{
		ID:           "p1",
		Name:         "text-embed-model",
		CanEmbedFlag: true,
		Backends:     []string{"b1"},
	}
	providerEmbedNoBackends := &modelprovider.MockProvider{
		ID:           "p2",
		Name:         "text-embed-model",
		CanEmbedFlag: true,
		Backends:     []string{}, // No backends
	}
	providerEmbedCannotEmbed := &modelprovider.MockProvider{
		ID:           "p4",
		Name:         "text-embed-model",
		CanEmbedFlag: false, // Cannot embed
		Backends:     []string{"b4"},
	}

	tests := []struct {
		name      string
		embedReq  llmresolver.ResolveEmbedRequest
		providers []modelprovider.Provider
		resolver  llmresolver.Resolver
		wantErr   error
		wantMsg   string
	}{
		{
			name:      "happy path - exact model match",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "text-embed-model"},
			providers: []modelprovider.Provider{providerEmbedOK},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   nil,
		},
		{
			name:      "error - model name required",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: ""},
			providers: []modelprovider.Provider{providerEmbedOK},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   fmt.Errorf("model name is required"),
			wantMsg:   "model name is required",
		},
		{
			name:      "error - no models available",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "text-embed-model"},
			providers: []modelprovider.Provider{},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   llmresolver.ErrNoAvailableModels,
		},
		{
			name:      "error - no satisfactory model (name mismatch)",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "non-existent-model"},
			providers: []modelprovider.Provider{providerEmbedOK},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   llmresolver.ErrNoSatisfactoryModel,
		},
		{
			name:      "error - no satisfactory model (capability mismatch)",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "text-embed-model"},
			providers: []modelprovider.Provider{providerEmbedCannotEmbed},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   llmresolver.ErrNoSatisfactoryModel,
		},
		{
			name:      "error - selected provider has no backends",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "text-embed-model"},
			providers: []modelprovider.Provider{providerEmbedNoBackends},
			resolver:  llmresolver.ResolveRandomly,
			// Error comes from selectRandomBackend called by ResolveRandomly
			wantErr: llmresolver.ErrNoSatisfactoryModel,
		},
		{
			name:      "multiple candidates - resolver selects one",
			embedReq:  llmresolver.ResolveEmbedRequest{ModelName: "text-embed-model"},
			providers: []modelprovider.Provider{providerEmbedOK, &modelprovider.MockProvider{ID: "p6", Name: "text-embed-model", CanEmbedFlag: true, Backends: []string{"b6"}}},
			resolver:  llmresolver.ResolveRandomly,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getModels := func(_ context.Context, providerType string) ([]modelprovider.Provider, error) {
				return tt.providers, nil
			}

			client, err := llmresolver.ResolveEmbed(context.Background(), tt.embedReq, getModels, tt.resolver)

			// Assertions
			if tt.wantErr != nil {
				if tt.wantMsg != "" {
					if err == nil {
						t.Errorf("ResolveEmbed() error = nil, want %q", tt.wantMsg)
					} else if err.Error() != tt.wantMsg {
						t.Errorf("ResolveEmbed() error = %q, want %q", err.Error(), tt.wantMsg)
					}
				} else {
					if !errors.Is(err, tt.wantErr) {
						t.Errorf("ResolveEmbed() error = %v, want %v", err, tt.wantErr)
					}
				}
				if client != nil {
					t.Errorf("ResolveEmbed() client = %v, want nil when error expected", client)
				}
			} else {
				// No error expected
				if err != nil {
					t.Errorf("ResolveEmbed() unexpected error = %v", err)
				}
				if client == nil {
					t.Errorf("ResolveEmbed() client is nil, want non-nil client")
				}
			}
		})
	}
}
