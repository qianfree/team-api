package shared

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

func TestParseThinkingSuffix(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		expectBase    string
		expectThink   bool
		expectNoThink bool
		expectEffort  string
	}{
		{
			name:        "thinking suffix",
			modelName:   "gpt-4-thinking",
			expectBase:  "gpt-4",
			expectThink: true,
		},
		{
			name:          "nothinking suffix",
			modelName:     "gpt-4-nothinking",
			expectBase:    "gpt-4",
			expectNoThink: true,
		},
		{
			name:         "low effort",
			modelName:    "gpt-4-low",
			expectBase:   "gpt-4",
			expectEffort: "low",
		},
		{
			name:         "high effort",
			modelName:    "gpt-4-high",
			expectBase:   "gpt-4",
			expectEffort: "high",
		},
		{
			name:       "no suffix",
			modelName:  "gpt-4",
			expectBase: "gpt-4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := ParseThinkingSuffix(tt.modelName)

			if info.BaseModel != tt.expectBase {
				t.Errorf("BaseModel = %q, want %q", info.BaseModel, tt.expectBase)
			}

			if info.IsThinking != tt.expectThink {
				t.Errorf("IsThinking = %v, want %v", info.IsThinking, tt.expectThink)
			}

			if info.IsNoThinking != tt.expectNoThink {
				t.Errorf("IsNoThinking = %v, want %v", info.IsNoThinking, tt.expectNoThink)
			}

			if info.EffortLevel != tt.expectEffort {
				t.Errorf("EffortLevel = %q, want %q", info.EffortLevel, tt.expectEffort)
			}
		})
	}
}

func TestApplyThinkingToClaude(t *testing.T) {
	tests := []struct {
		name           string
		info           ThinkingInfo
		opts           convmeta.ClaudeOptions
		maxTokens      *uint
		expectThinking bool
		expectBudget   bool
	}{
		{
			name: "adapter disabled",
			info: ThinkingInfo{IsThinking: true},
			opts: convmeta.ClaudeOptions{
				ThinkingAdapterEnabled: false,
			},
			expectThinking: false,
		},
		{
			name: "thinking enabled without budget",
			info: ThinkingInfo{IsThinking: true},
			opts: convmeta.ClaudeOptions{
				ThinkingAdapterEnabled: true,
			},
			expectThinking: true,
			expectBudget:   false,
		},
		{
			name:      "thinking enabled with budget",
			info:      ThinkingInfo{IsThinking: true},
			maxTokens: uintPtr(1000),
			opts: convmeta.ClaudeOptions{
				ThinkingAdapterEnabled:                true,
				ThinkingAdapterBudgetTokensPercentage: 0.2,
			},
			expectThinking: true,
			expectBudget:   true,
		},
		{
			name: "nothinking suppresses thinking",
			info: ThinkingInfo{IsNoThinking: true},
			opts: convmeta.ClaudeOptions{
				ThinkingAdapterEnabled: true,
			},
			expectThinking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &dto.ClaudeRequest{
				MaxTokens: tt.maxTokens,
			}

			ApplyThinkingToClaude(req, tt.info, tt.opts)

			if tt.expectThinking {
				if req.Thinking == nil {
					t.Error("Expected Thinking to be set, got nil")
				} else {
					if req.Thinking.Type != "enabled" {
						t.Errorf("Expected Thinking.Type = 'enabled', got %q", req.Thinking.Type)
					}

					if tt.expectBudget {
						if req.Thinking.BudgetTokens == nil {
							t.Error("Expected BudgetTokens to be set, got nil")
						}
					}
				}
			} else {
				if req.Thinking != nil {
					t.Error("Expected Thinking to be nil, got non-nil")
				}
			}
		})
	}
}

func TestApplyThinkingToGemini(t *testing.T) {
	tests := []struct {
		name           string
		info           ThinkingInfo
		opts           convmeta.GeminiOptions
		maxTokens      *uint
		expectThinking bool
		expectBudget   bool
	}{
		{
			name: "adapter disabled",
			info: ThinkingInfo{IsThinking: true},
			opts: convmeta.GeminiOptions{
				ThinkingAdapterEnabled: false,
			},
			expectThinking: false,
		},
		{
			name: "thinking enabled with effort",
			info: ThinkingInfo{EffortLevel: "high"},
			opts: convmeta.GeminiOptions{
				ThinkingAdapterEnabled: true,
			},
			expectThinking: true,
		},
		{
			name:      "thinking with budget",
			info:      ThinkingInfo{IsThinking: true},
			maxTokens: uintPtr(2000),
			opts: convmeta.GeminiOptions{
				ThinkingAdapterEnabled:                true,
				ThinkingAdapterBudgetTokensPercentage: 0.3,
			},
			expectThinking: true,
			expectBudget:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &dto.GeminiGenerationConfig{
				MaxOutputTokens: tt.maxTokens,
			}

			ApplyThinkingToGemini(config, tt.info, tt.opts)

			if tt.expectThinking {
				if config.ThinkingConfig == nil {
					t.Error("Expected ThinkingConfig to be set, got nil")
				} else {
					if !config.ThinkingConfig.IncludeThoughts {
						t.Error("Expected IncludeThoughts = true, got false")
					}

					if tt.expectBudget {
						if config.ThinkingConfig.ThoughtBudget == nil {
							t.Error("Expected ThoughtBudget to be set, got nil")
						}
					}
				}
			} else {
				if config.ThinkingConfig != nil {
					t.Error("Expected ThinkingConfig to be nil, got non-nil")
				}
			}
		})
	}
}

func uintPtr(i uint) *uint {
	return &i
}

func intPtr(i int) *int {
	return &i
}
