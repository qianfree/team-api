// Package shared 提供格式转换相关的可复用映射函数。
// 这些函数从 relay/channel 适配器中抽取出来，已彼此独立。
package shared

import (
	"encoding/json"

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
		// JSON 直解形态：元素是 map[string]any 而非 typed 结构，逐项按 map 读取
		for _, item := range v {
			switch p := item.(type) {
			case dto.ContentPart:
				if p.Type == "text" {
					return p.Text
				}
			case map[string]any:
				if p["type"] == "text" {
					if t, ok := p["text"].(string); ok {
						return t
					}
				}
			}
		}
	}
	return ""
}

// NormalizeContentParts 将任意形态的 OpenAI content 归一为 []dto.ContentPart。
// Content 字段类型为 any，存在两种运行时形态：JSON 直解产出的 []any（元素 map[string]any，
// 真实客户端流量的形态）与链式转换第一跳产出的 []dto.ContentPart（dispatch 原样传对象、
// 无 JSON 往返）。消费 content 的 type switch 必须同时覆盖两种形态，漏一种即静默丢内容。
func NormalizeContentParts(content any) []dto.ContentPart {
	switch v := content.(type) {
	case []dto.ContentPart:
		return v
	case []any:
		parts := make([]dto.ContentPart, 0, len(v))
		for _, item := range v {
			switch p := item.(type) {
			case dto.ContentPart:
				parts = append(parts, p)
			case map[string]any:
				b, err := json.Marshal(p)
				if err != nil {
					continue
				}
				var cp dto.ContentPart
				if err := json.Unmarshal(b, &cp); err == nil {
					parts = append(parts, cp)
				}
			}
		}
		return parts
	}
	return nil
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
				url := block.Source.URL
				if url == "" && block.Source.Data != "" && block.Source.MediaType != "" {
					// base64 source 还原为数据 URL（裸 base64 缺少 data: 前缀，OpenAI 客户端不认）
					url = "data:" + block.Source.MediaType + ";base64," + block.Source.Data
				}
				parts = append(parts, dto.ContentPart{
					Type:     "image_url",
					ImageURL: &dto.ImageURL{URL: url},
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
		// 抽取媒体类型和数据；无 ";base64," 标记的畸形 data URL 走 url 分支（base64 空 data 必 400）
		if idx := findSubstring(url, ";base64,"); idx > 0 {
			mediaType := url[5:idx]
			data := url[idx+8:]
			source.MediaType = mediaType
			source.Data = data
			return source
		}
	}

	// 远程 URL：Anthropic URL source 要求 url 字段（填 data 字段上游直接 400）
	source.Type = "url"
	source.Data = ""
	source.URL = url
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
