package shared

import (
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

func TestMapTextContent(t *testing.T) {
	tests := []struct {
		name     string
		content  any
		expected string
	}{
		{
			name:     "string content",
			content:  "Hello world",
			expected: "Hello world",
		},
		{
			name: "content parts with text",
			content: []dto.ContentPart{
				{Type: "text", Text: "First text"},
				{Type: "image_url", ImageURL: &dto.ImageURL{URL: "http://example.com/image.png"}},
			},
			expected: "First text",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name:     "nil content",
			content:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapTextContent(tt.content)
			if result != tt.expected {
				t.Errorf("MapTextContent() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMapOpenAIContentPartsToClaude(t *testing.T) {
	tests := []struct {
		name     string
		parts    []dto.ContentPart
		expected int // 期望的 block 数量
	}{
		{
			name: "text part",
			parts: []dto.ContentPart{
				{Type: "text", Text: "Hello"},
			},
			expected: 1,
		},
		{
			name: "text and image parts",
			parts: []dto.ContentPart{
				{Type: "text", Text: "Hello"},
				{Type: "image_url", ImageURL: &dto.ImageURL{URL: "data:image/png;base64,abc123"}},
			},
			expected: 2,
		},
		{
			name:     "empty parts",
			parts:    []dto.ContentPart{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapOpenAIContentPartsToClaude(tt.parts)
			if len(result) != tt.expected {
				t.Errorf("MapOpenAIContentPartsToClaude() returned %d blocks, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestMapClaudeContentToOpenAI(t *testing.T) {
	tests := []struct {
		name        string
		blocks      []dto.ClaudeContentBlock
		expectType  string // "string" or "array"
		expectEmpty bool
	}{
		{
			name: "single text block",
			blocks: []dto.ClaudeContentBlock{
				{Type: "text", Text: strPtr("Hello")},
			},
			expectType: "string",
		},
		{
			name: "multiple blocks",
			blocks: []dto.ClaudeContentBlock{
				{Type: "text", Text: strPtr("Hello")},
				{Type: "text", Text: strPtr("World")},
			},
			expectType: "array",
		},
		{
			name:        "empty blocks",
			blocks:      []dto.ClaudeContentBlock{},
			expectEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MapClaudeContentToOpenAI(tt.blocks)

			if tt.expectEmpty {
				if str, ok := result.(string); !ok || str != "" {
					t.Errorf("Expected empty string, got %v", result)
				}
				return
			}

			switch tt.expectType {
			case "string":
				if _, ok := result.(string); !ok {
					t.Errorf("Expected string type, got %T", result)
				}
			case "array":
				if _, ok := result.([]dto.ContentPart); !ok {
					t.Errorf("Expected []dto.ContentPart type, got %T", result)
				}
			}
		})
	}
}

func strPtr(s string) *string {
	return &s
}
