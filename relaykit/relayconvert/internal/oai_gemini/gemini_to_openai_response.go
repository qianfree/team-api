package oai_gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// GeminiToOpenAIResponseConverter 将 Gemini Generate Content 响应转换为 OpenAI Chat Completions 响应。
type GeminiToOpenAIResponseConverter struct{}

func (c *GeminiToOpenAIResponseConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToOAIChat
}

func (c *GeminiToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *GeminiToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

func (c *GeminiToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	geminiResp, ok := response.(*dto.GeminiChatResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeminiChatResponse, got %T", response)
	}

	// 生成响应 ID
	responseID := fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())

	modelName := geminiResp.ModelName
	if modelName == "" && info != nil {
		modelName = info.GetOriginModelName()
	}
	if modelName == "" {
		modelName = "gemini-pro"
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      responseID,
		Object:  "chat.completion",
		Created: 0,
		Model:   modelName,
		Choices: make([]dto.Choice, 0),
	}

	// 检查 prompt feedback 中的安全拦截
	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason)
	}

	// 将 candidates 转换为 choices
	if len(geminiResp.Candidates) == 0 {
		return openaiResp, nil
	}

	candidate := geminiResp.Candidates[0]
	choice := dto.Choice{
		Index:        candidate.Index,
		FinishReason: mapGeminiFinishReason(candidate.FinishReason),
	}

	if candidate.Content != nil {
		var textParts []string
		var thinkingParts []string
		var toolCalls []dto.ToolCall
		toolIdx := 0

		for _, part := range candidate.Content.Parts {
			isThought := part.Thought != nil && *part.Thought

			// 文本内容
			if part.Text != "" {
				if isThought {
					thinkingParts = append(thinkingParts, part.Text)
				} else {
					textParts = append(textParts, part.Text)
				}
			}

			// 内联图片数据
			if part.InlineData != nil {
				imageMarkdown := fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
				textParts = append(textParts, imageMarkdown)
			}

			// 文件数据
			if part.FileData != nil {
				fileMarkdown := fmt.Sprintf("[file](%s)", part.FileData.FileURI)
				textParts = append(textParts, fileMarkdown)
			}

			// 可执行代码
			if part.ExecutableCode != nil {
				codeBlock := fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code)
				textParts = append(textParts, codeBlock)
			}

			// 代码执行结果
			if part.CodeExecutionResult != nil {
				resultText := fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output)
				textParts = append(textParts, resultText)
			}

			// 函数调用
			if part.FunctionCall != nil {
				argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
				toolCalls = append(toolCalls, dto.ToolCall{
					ID:   fmt.Sprintf("call_%s_%d", responseID, toolIdx),
					Type: "function",
					Function: dto.FunctionCall{
						Name:      part.FunctionCall.FunctionName,
						Arguments: string(argsJSON),
					},
				})
				toolIdx++
			}
		}

		message := dto.Message{
			Role:    "assistant",
			Content: strings.Join(textParts, ""),
		}

		// 添加 thinking 内容
		if len(thinkingParts) > 0 {
			joined := strings.Join(thinkingParts, "\n")
			message.ReasoningContent = &joined
		}

		// 添加工具调用
		if len(toolCalls) > 0 {
			message.ToolCalls = toolCalls
			choice.FinishReason = "tool_calls"
		}

		choice.Message = message
	}

	openaiResp.Choices = append(openaiResp.Choices, choice)

	// 转换 usage
	if geminiResp.UsageMetadata != nil {
		openaiResp.Usage = dto.UsageWithDetails{
			PromptTokens:     geminiResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: geminiResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      geminiResp.UsageMetadata.TotalTokenCount,
		}
	}

	return openaiResp, nil
}

// 辅助函数

func mapGeminiFinishReason(reason string) string {
	switch reason {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "RECITATION":
		return "content_filter"
	case "OTHER":
		return "stop"
	case "BLOCKLIST":
		return "content_filter"
	case "PROHIBITED_CONTENT":
		return "content_filter"
	case "SPII":
		return "content_filter"
	default:
		if reason == "" {
			return "stop"
		}
		return "stop"
	}
}
