package handler

import (
	"context"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

type modelMappingStub struct {
	models map[string]bool
	calls  []string
}

func (s *modelMappingStub) GetModelMapping(_ context.Context, modelName string) (string, string, error) {
	s.calls = append(s.calls, modelName)
	if !s.models[modelName] {
		return "", "", common.ErrModelNotFound
	}
	return modelName, "chat", nil
}

func TestResolveRelayModel(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		models     map[string]bool
		wantLookup string
		wantEffort string
		wantCalls  []string
		wantErr    bool
	}{
		{
			name:       "literal model ending in effort suffix wins",
			model:      "qwen3.8-max",
			models:     map[string]bool{"qwen3.8-max": true, "qwen3.8": true},
			wantLookup: "qwen3.8-max",
			wantCalls:  []string{"qwen3.8-max"},
		},
		{
			name:       "virtual effort suffix falls back to base model",
			model:      "o3-max",
			models:     map[string]bool{"o3": true},
			wantLookup: "o3",
			wantEffort: "max",
			wantCalls:  []string{"o3-max", "o3"},
		},
		{
			name:       "model without suffix resolves literally",
			model:      "gpt-4o",
			models:     map[string]bool{"gpt-4o": true},
			wantLookup: "gpt-4o",
			wantCalls:  []string{"gpt-4o"},
		},
		{
			name:      "missing literal and base model returns error",
			model:     "missing-high",
			models:    map[string]bool{},
			wantCalls: []string{"missing-high", "missing"},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &modelMappingStub{models: tt.models}
			lookup, thinking, err := resolveRelayModel(t.Context(), provider, tt.model)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveRelayModel() error = %v, wantErr %v", err, tt.wantErr)
			}
			if lookup != tt.wantLookup {
				t.Errorf("lookup model = %q, want %q", lookup, tt.wantLookup)
			}
			if !tt.wantErr {
				if thinking == nil {
					t.Fatal("thinking info is nil")
				}
				if thinking.BaseModel != tt.wantLookup {
					t.Errorf("thinking base model = %q, want %q", thinking.BaseModel, tt.wantLookup)
				}
				if thinking.EffortLevel != tt.wantEffort {
					t.Errorf("effort level = %q, want %q", thinking.EffortLevel, tt.wantEffort)
				}
			}
			if len(provider.calls) != len(tt.wantCalls) {
				t.Fatalf("lookup calls = %v, want %v", provider.calls, tt.wantCalls)
			}
			for i := range tt.wantCalls {
				if provider.calls[i] != tt.wantCalls[i] {
					t.Errorf("lookup calls = %v, want %v", provider.calls, tt.wantCalls)
					break
				}
			}
		})
	}
}
