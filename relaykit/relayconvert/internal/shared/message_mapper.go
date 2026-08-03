// Package shared 提供格式转换相关的可复用映射函数。
// 这些函数从 relay/channel 适配器中抽取出来，已彼此独立。
package shared

import (
	"github.com/qianfree/team-api/relaykit/dto"
)

// MapTextContent 从多种内容格式中抽取纯文本内容。
// 未找到文本时返回空字符串。
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

// MapOpenAIContentPartsToClaude 将 OpenAI ContentPart[] 转换为 Claude ContentBlock[]。
// 处理 text、image_url 等多模态内容类型。
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

			// 其他内容类型可在此处扩展
		}
	}

	return blocks
}

// MapClaudeContentToOpenAI 将 Claude ContentBlock[] 转换为 OpenAI 消息内容。
// 纯文本时返回单个字符串；多模态时返回 ContentPart[]。
func MapClaudeContentToOpenAI(blocks []dto.ClaudeContentBlock) any {
	if len(blocks) == 0 {
		return ""
	}

	// 仅单个文本块时，按字符串返回
	if len(blocks) == 1 && blocks[0].Type == "text" {
		if blocks[0].Text != nil {
			return *blocks[0].Text
		}
		return ""
	}

	// 否则作为 ContentPart 数组返回
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
						URL: block.Source.Data, // Base64 数据 URL
					},
				})
			}
		}
	}

	return parts
}

// MapOpenAIImageToClaudeSource 将 OpenAI ImageURL 转换为 Claude Source 格式。
func MapOpenAIImageToClaudeSource(imageURL dto.ImageURL) dto.ClaudeSource {
	source := dto.ClaudeSource{
		Type: "base64",
	}

	// 解析数据 URL：data:image/png;base64,xxxxx
	url := imageURL.URL
	if len(url) > 5 && url[:5] == "data:" {
		// 抽取媒体类型和数据
		if idx := findSubstring(url, ";base64,"); idx > 0 {
			mediaType := url[5:idx]
			data := url[idx+8:]
			source.MediaType = mediaType
			source.Data = data
		}
	} else {
		// HTTP URL —— Claude 不直接支持，需要自行抓取
		source.Type = "url"
		source.Data = url
	}

	return source
}

// findSubstring 是用于查找子串索引的辅助函数
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
