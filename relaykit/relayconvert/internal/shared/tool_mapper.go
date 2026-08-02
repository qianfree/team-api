package shared

import (
	"encoding/json"

	"github.com/qianfree/team-api/relaykit/dto"
)

// MapOpenAIToolsToClaudeTools 将 OpenAI Tool[] 转换为 Claude ClaudeTool[]。
func MapOpenAIToolsToClaudeTools(tools []dto.Tool) []dto.ClaudeTool {
	claudeTools := make([]dto.ClaudeTool, 0, len(tools))

	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}

		claudeTool := dto.ClaudeTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
		}

		// 转换参数（JSON Schema）
		if tool.Function.Parameters != nil {
			claudeTool.InputSchema = tool.Function.Parameters
		}

		claudeTools = append(claudeTools, claudeTool)
	}

	return claudeTools
}

// MapClaudeToolsToOpenAITools 将 Claude ClaudeTool[] 转换为 OpenAI Tool[]。
func MapClaudeToolsToOpenAITools(tools []dto.ClaudeTool) []dto.Tool {
	openaiTools := make([]dto.Tool, 0, len(tools))

	for _, tool := range tools {
		openaiTool := dto.Tool{
			Type: "function",
			Function: dto.FunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}

		openaiTools = append(openaiTools, openaiTool)
	}

	return openaiTools
}

// MapClaudeToolCallsToOpenAI 将 Claude tool_use ContentBlock[] 转换为 OpenAI ToolCall[]。
func MapClaudeToolCallsToOpenAI(blocks []dto.ClaudeContentBlock) []dto.ToolCall {
	toolCalls := make([]dto.ToolCall, 0)

	for _, block := range blocks {
		if block.Type != "tool_use" {
			continue
		}

		// 将 Input 序列化为 JSON 字符串
		arguments := "{}"
		if block.Input != nil {
			if data, err := json.Marshal(block.Input); err == nil {
				arguments = string(data)
			}
		}

		toolCalls = append(toolCalls, dto.ToolCall{
			ID:   block.ID,
			Type: "function",
			Function: dto.FunctionCall{
				Name:      block.Name,
				Arguments: arguments,
			},
		})
	}

	return toolCalls
}

// MapOpenAIToolCallsToClaude 将 OpenAI ToolCall[] 转换为 Claude tool_use ContentBlock[]。
func MapOpenAIToolCallsToClaude(toolCalls []dto.ToolCall) []dto.ClaudeContentBlock {
	blocks := make([]dto.ClaudeContentBlock, 0, len(toolCalls))

	for _, tc := range toolCalls {
		if tc.Type != "function" {
			continue
		}

		// 将 Arguments JSON 字符串解析为 map
		var input map[string]any
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
		}
		if input == nil {
			input = make(map[string]any)
		}

		blocks = append(blocks, dto.ClaudeContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: input,
		})
	}

	return blocks
}
