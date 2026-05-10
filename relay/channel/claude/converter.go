package claude

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
)

// ConvertOpenAIToClaude 将 OpenAI 格式请求转换为 Claude Messages API 格式。
func ConvertOpenAIToClaude(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	var openaiReq dto.GeneralOpenAIRequest
	if err := json.Unmarshal(requestBody, &openaiReq); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}

	claudeReq := dto.ClaudeRequest{
		Model:    info.ChannelMeta.UpstreamModelName,
		Messages: make([]dto.ClaudeMessage, 0, len(openaiReq.Messages)),
	}

	if openaiReq.MaxTokens != nil {
		maxTokens := uint(*openaiReq.MaxTokens)
		claudeReq.MaxTokens = &maxTokens
	} else if openaiReq.MaxCompletionTokens != nil {
		maxTokens := uint(*openaiReq.MaxCompletionTokens)
		claudeReq.MaxTokens = &maxTokens
	} else {
		defaultMax := uint(4096)
		claudeReq.MaxTokens = &defaultMax
	}

	if openaiReq.Temperature != nil {
		claudeReq.Temperature = openaiReq.Temperature
	}
	if openaiReq.TopP != nil {
		claudeReq.TopP = openaiReq.TopP
	}
	if openaiReq.TopK != nil {
		claudeReq.TopK = openaiReq.TopK
	}
	if openaiReq.Stop != nil {
		if stops, ok := openaiReq.Stop.([]any); ok {
			for _, s := range stops {
				if str, ok := s.(string); ok {
					claudeReq.StopSequences = append(claudeReq.StopSequences, str)
				}
			}
		} else if str, ok := openaiReq.Stop.(string); ok {
			claudeReq.StopSequences = append(claudeReq.StopSequences, str)
		}
	}
	if openaiReq.Stream != nil {
		claudeReq.Stream = openaiReq.Stream
	}

	// reasoning_effort → thinking
	if openaiReq.ReasoningEffort != "" {
		claudeReq.Thinking = o2cConvertReasoningEffort(openaiReq.ReasoningEffort)
		if claudeReq.MaxTokens != nil && *claudeReq.MaxTokens < 16384 {
			v := uint(16384)
			claudeReq.MaxTokens = &v
		}
	}

	if len(openaiReq.Tools) > 0 {
		claudeReq.Tools = make([]dto.ClaudeTool, 0, len(openaiReq.Tools))
		for _, t := range openaiReq.Tools {
			if t.Type == "function" {
				claudeReq.Tools = append(claudeReq.Tools, dto.ClaudeTool{
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: t.Function.Parameters,
				})
			}
		}
	}

	claudeReq.ToolChoice = o2cConvertToolChoice(openaiReq.ToolChoice, openaiReq.ParallelToolCalls)

	var systemBlocks []dto.ClaudeContentBlock
	var lastRole string

	for _, msg := range openaiReq.Messages {
		content := msg.Content
		if content == nil {
			content = ""
		}

		switch msg.Role {
		case "system", "developer":
			text := o2cExtractTextContent(content)
			if text != "" {
				systemBlocks = append(systemBlocks, dto.ClaudeContentBlock{
					Type: "text",
					Text: &text,
				})
			}

		case "user":
			blocks := o2cConvertUserMessage(content)
			if lastRole == "user" && len(claudeReq.Messages) > 0 {
				lastMsg := &claudeReq.Messages[len(claudeReq.Messages)-1]
				lastMsg.Content = o2cMergeContents(lastMsg.Content, blocks)
			} else {
				claudeReq.Messages = append(claudeReq.Messages, dto.ClaudeMessage{
					Role:    "user",
					Content: blocks,
				})
			}
			lastRole = "user"

		case "assistant":
			blocks := o2cConvertAssistantMessage(msg)
			if lastRole == "assistant" && len(claudeReq.Messages) > 0 {
				lastMsg := &claudeReq.Messages[len(claudeReq.Messages)-1]
				lastMsg.Content = o2cMergeContents(lastMsg.Content, blocks)
			} else {
				claudeReq.Messages = append(claudeReq.Messages, dto.ClaudeMessage{
					Role:    "assistant",
					Content: blocks,
				})
			}
			lastRole = "assistant"

		case "tool":
			toolResult := dto.ClaudeContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   o2cExtractTextContent(content),
			}
			if lastRole == "user" && len(claudeReq.Messages) > 0 {
				lastMsg := &claudeReq.Messages[len(claudeReq.Messages)-1]
				lastMsg.Content = o2cMergeContents(lastMsg.Content, []dto.ClaudeContentBlock{toolResult})
			} else {
				claudeReq.Messages = append(claudeReq.Messages, dto.ClaudeMessage{
					Role:    "user",
					Content: []dto.ClaudeContentBlock{toolResult},
				})
			}
			lastRole = "user"
		}
	}

	if len(claudeReq.Messages) > 0 && claudeReq.Messages[0].Role != "user" {
		claudeReq.Messages = append([]dto.ClaudeMessage{
			{Role: "user", Content: " "},
		}, claudeReq.Messages...)
	}

	if len(systemBlocks) > 0 {
		if len(systemBlocks) == 1 {
			claudeReq.System = systemBlocks[0].Text
		} else {
			claudeReq.System = systemBlocks
		}
	}

	result, err := json.Marshal(claudeReq)
	if err != nil {
		return nil, fmt.Errorf("marshal claude request failed: %w", err)
	}

	return bytes.NewReader(result), nil
}

// ConvertGeminiToClaude 将 Gemini 格式请求转换为 Claude Messages API 格式。
// 通过 OpenAI 作为中间格式：Gemini → OpenAI → Claude
func ConvertGeminiToClaude(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	openaiBody, err := openai.ConvertGeminiToOpenAI(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("gemini to openai conversion failed: %w", err)
	}
	openaiBytes, err := io.ReadAll(openaiBody)
	if err != nil {
		return nil, fmt.Errorf("read openai intermediate body failed: %w", err)
	}
	return ConvertOpenAIToClaude(openaiBytes, info)
}

// ConvertResponsesToClaude 将 OpenAI Responses API 格式请求转换为 Claude Messages API 格式。
// 通过 OpenAI 作为中间格式：Responses → OpenAI → Claude
func ConvertResponsesToClaude(requestBody []byte, info *common.RelayInfo) (io.Reader, error) {
	openaiBody, err := openai.ConvertResponsesToOpenAI(requestBody, info)
	if err != nil {
		return nil, fmt.Errorf("responses to openai conversion failed: %w", err)
	}
	openaiBytes, err := io.ReadAll(openaiBody)
	if err != nil {
		return nil, fmt.Errorf("read openai intermediate body failed: %w", err)
	}
	return ConvertOpenAIToClaude(openaiBytes, info)
}

func o2cConvertToolChoice(toolChoice any, parallelToolCalls *bool) any {
	if toolChoice == nil {
		if parallelToolCalls != nil && !*parallelToolCalls {
			return map[string]any{
				"type":                      "auto",
				"disable_parallel_tool_use": true,
			}
		}
		return nil
	}

	switch v := toolChoice.(type) {
	case string:
		switch v {
		case "auto":
			return map[string]any{"type": "auto"}
		case "none":
			return nil
		case "required":
			return map[string]any{"type": "any"}
		}
	case map[string]any:
		if v["type"] == "function" {
			if fn, ok := v["function"].(map[string]any); ok {
				if name, ok := fn["name"].(string); ok {
					return map[string]any{"type": "tool", "name": name}
				}
			}
		}
	}
	return toolChoice
}

func o2cMergeContents(existing any, newContent any) any {
	existingBlocks := o2cContentToBlocks(existing)
	newBlocks := o2cContentToBlocks(newContent)
	merged := append(existingBlocks, newBlocks...)
	if len(merged) == 1 && merged[0].Type == "text" {
		return merged[0].Text
	}
	return merged
}

func o2cContentToBlocks(content any) []dto.ClaudeContentBlock {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []dto.ClaudeContentBlock{{Type: "text", Text: &v}}
	case []dto.ClaudeContentBlock:
		return v
	default:
		if content == nil {
			return nil
		}
		return []dto.ClaudeContentBlock{{Type: "text", Text: strPtr(fmt.Sprintf("%v", v))}}
	}
}

func o2cExtractTextContent(content any) string {
	switch v := content.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if m["type"] == "text" {
					if text, ok := m["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return o2cJoinTextParts(parts)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func o2cConvertUserMessage(content any) any {
	switch v := content.(type) {
	case string:
		if v == "" {
			return []dto.ClaudeContentBlock{{Type: "text", Text: strPtr(" ")}}
		}
		return v
	case []any:
		blocks := make([]dto.ClaudeContentBlock, 0, len(v))
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			switch m["type"] {
			case "text":
				if text, ok := m["text"].(string); ok {
					blocks = append(blocks, dto.ClaudeContentBlock{Type: "text", Text: &text})
				}
			case "image_url":
				if imageURL, ok := m["image_url"].(map[string]any); ok {
					if url, ok := imageURL["url"].(string); ok {
						if len(url) > 5 && url[:5] == "data:" {
							block, ok := o2cParseDataURL(url)
							if ok {
								blocks = append(blocks, block)
								continue
							}
						}
						if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
							blocks = append(blocks, dto.ClaudeContentBlock{
								Type:   "image",
								Source: &dto.ClaudeSource{Type: "url", URL: url},
							})
						} else {
							blocks = append(blocks, dto.ClaudeContentBlock{Type: "text", Text: &url})
						}
					}
				}
			}
		}
		if len(blocks) == 0 {
			return []dto.ClaudeContentBlock{{Type: "text", Text: strPtr(" ")}}
		}
		if len(blocks) == 1 && blocks[0].Type == "text" {
			return blocks[0].Text
		}
		return blocks
	default:
		return fmt.Sprintf("%v", v)
	}
}

func o2cConvertAssistantMessage(msg dto.Message) any {
	blocks := make([]dto.ClaudeContentBlock, 0)

	if msg.ReasoningContent != nil && *msg.ReasoningContent != "" {
		blocks = append(blocks, dto.ClaudeContentBlock{
			Type:     "thinking",
			Thinking: msg.ReasoningContent,
			// Note: Anthropic officially requires a valid signature for thinking blocks in history.
			// Since OpenAI format doesn't have a dedicated signature field, we leave it empty here.
			// If API rejects this, it may need to be stored in Message.Annotations or elsewhere in the future.
		})
	}

	text := o2cExtractTextContent(msg.Content)
	if text != "" {
		blocks = append(blocks, dto.ClaudeContentBlock{Type: "text", Text: &text})
	}

	for _, tc := range msg.ToolCalls {
		var inputObj any
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
			inputObj = map[string]any{}
		}
		blocks = append(blocks, dto.ClaudeContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: inputObj,
		})
	}

	if len(blocks) == 0 {
		return []dto.ClaudeContentBlock{{Type: "text", Text: strPtr(" ")}}
	}
	if len(blocks) == 1 && blocks[0].Type == "text" {
		return blocks[0].Text
	}
	return blocks
}

func o2cParseDataURL(dataURL string) (dto.ClaudeContentBlock, bool) {
	if len(dataURL) < 11 {
		return dto.ClaudeContentBlock{}, false
	}
	rest := dataURL[5:]
	semicolonIdx := -1
	for i := 0; i < len(rest); i++ {
		if rest[i] == ';' {
			semicolonIdx = i
			break
		}
	}
	if semicolonIdx == -1 {
		return dto.ClaudeContentBlock{}, false
	}

	mediaType := rest[:semicolonIdx]
	afterSemicolon := rest[semicolonIdx+1:]
	if len(afterSemicolon) < 7 || afterSemicolon[:7] != "base64," {
		return dto.ClaudeContentBlock{}, false
	}
	data := afterSemicolon[7:]

	contentType := "image"
	if mediaType == "application/pdf" {
		contentType = "document"
	}

	return dto.ClaudeContentBlock{
		Type: contentType,
		Source: &dto.ClaudeSource{
			Type:      "base64",
			MediaType: mediaType,
			Data:      data,
		},
	}, true
}

func o2cJoinTextParts(parts []string) string {
	result := make([]byte, 0, 64)
	for i, p := range parts {
		if i > 0 {
			result = append(result, '\n')
		}
		result = append(result, p...)
	}
	return string(result)
}

// o2cConvertReasoningEffort 将 OpenAI reasoning_effort 转换为 Claude thinking 配置
func o2cConvertReasoningEffort(effort string) *dto.ClaudeThinking {
	var budget int
	switch effort {
	case "low":
		budget = 1024
	case "medium":
		budget = 8192
	case "high":
		budget = 32768
	default:
		budget = 8192
	}
	return &dto.ClaudeThinking{
		Type:         "enabled",
		BudgetTokens: &budget,
	}
}

// strPtr 返回 string 的指针
func strPtr(v string) *string { return &v }
