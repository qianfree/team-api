// Package shared provides reusable mapping functions for format conversion.
// These functions are extracted from relay/channel adapters and made independent.
package shared

import (
	"github.com/qianfree/team-api/relaykit/dto"
)

// MapTextContent extracts plain text content from various content formats.
// Returns empty string if no text is found.
func MapTextContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []dto.ContentPart:
		for _, part := range v {
			if part.Type == "text" {
				return part.Text
			}
		}
	case []any:
		for _, item := range v {
			if part, ok := item.(dto.ContentPart); ok && part.Type == "text" {
				return part.Text
			}
		}
	}
	return ""
}

// MapOpenAIContentPartsToClaude converts OpenAI ContentPart[] to Claude ContentBlock[].
// Handles text, image_url, and other multimodal content types.
func MapOpenAIContentPartsToClaude(parts []dto.ContentPart) []dto.ClaudeContentBlock {
	blocks := make([]dto.ClaudeContentBlock, 0, len(parts))

	for _, part := range parts {
		switch part.Type {
		case "text":
			text := part.Text
			blocks = append(blocks, dto.ClaudeContentBlock{
				Type: "text",
				Text: &text,
			})

		case "image_url":
			if part.ImageURL != nil {
				source := MapOpenAIImageToClaudeSource(*part.ImageURL)
				blocks = append(blocks, dto.ClaudeContentBlock{
					Type:   "image",
					Source: &source,
				})
			}

		// Other content types can be added here
		}
	}

	return blocks
}

// MapClaudeContentToOpenAI converts Claude ContentBlock[] to OpenAI message content.
// For simple text, returns a single string; for multimodal, returns ContentPart[].
func MapClaudeContentToOpenAI(blocks []dto.ClaudeContentBlock) any {
	if len(blocks) == 0 {
		return ""
	}

	// If only one text block, return as string
	if len(blocks) == 1 && blocks[0].Type == "text" {
		if blocks[0].Text != nil {
			return *blocks[0].Text
		}
		return ""
	}

	// Otherwise, return as ContentPart array
	parts := make([]dto.ContentPart, 0, len(blocks))
	for _, block := range blocks {
		switch block.Type {
		case "text":
			text := ""
			if block.Text != nil {
				text = *block.Text
			}
			parts = append(parts, dto.ContentPart{
				Type: "text",
				Text: text,
			})

		case "image":
			if block.Source != nil {
				parts = append(parts, dto.ContentPart{
					Type: "image_url",
					ImageURL: &dto.ImageURL{
						URL: block.Source.Data, // Base64 data URL
					},
				})
			}
		}
	}

	return parts
}

// MapOpenAIImageToClaudeSource converts OpenAI ImageURL to Claude Source format.
func MapOpenAIImageToClaudeSource(imageURL dto.ImageURL) dto.ClaudeSource {
	source := dto.ClaudeSource{
		Type: "base64",
	}

	// Parse data URL: data:image/png;base64,xxxxx
	url := imageURL.URL
	if len(url) > 5 && url[:5] == "data:" {
		// Extract media type and data
		if idx := findSubstring(url, ";base64,"); idx > 0 {
			mediaType := url[5:idx]
			data := url[idx+8:]
			source.MediaType = mediaType
			source.Data = data
		}
	} else {
		// HTTP URL - not directly supported by Claude, would need to fetch
		source.Type = "url"
		source.Data = url
	}

	return source
}

// findSubstring is a helper to find substring index
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
