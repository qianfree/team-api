package native_responses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// GeminiToResponsesResponseConverter 将 Gemini GenerateContent 非流式响应转换为 OpenAI Responses 响应。
//
// 用量口径：Responses 的客户端可见 usage 与 Gemini 上游的计费口径**恰好一致**
// （都是 OpenAI 语义：input 含缓存、output 含思考），故两者共用 geminiUsageToDetails。
//
// 协议落差：
//   - Gemini 的 functionCall 未必带 id，Responses 的 function_call 必须有 call_id → 缺失时合成。
//   - 思考内容在 Responses 非流式响应里没有对应物（沿用 claude→responses 的口径跳过）。
type GeminiToResponsesResponseConverter struct{}

func (c *GeminiToResponsesResponseConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToOAIResponses
}

func (c *GeminiToResponsesResponseConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToResponsesResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *GeminiToResponsesResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 *dto.GeminiChatResponse 转换为 Responses 响应对象（map 构造，与旧桥一致）。
// promptFeedback.blockReason 命中（Gemini 安全过滤）时返回错误。
func (c *GeminiToResponsesResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	geminiResp, ok := response.(*dto.GeminiChatResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.GeminiChatResponse, got %T", response)
	}

	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, fmt.Errorf("request blocked by Gemini safety filter: %s: %w", geminiResp.PromptFeedback.BlockReason, relayconvert.ErrContentBlocked)
	}

	idBase := fmt.Sprintf("%d", time.Now().UnixNano())
	output := buildResponsesOutputFromGemini(geminiResp, idBase)

	modelName := geminiResp.ModelName
	if modelName == "" || isModelMapped(info) {
		modelName = originModelName(info)
	}
	createdAt := int(time.Now().Unix())
	completedAt := createdAt

	usage := geminiUsageToDetails(geminiResp.UsageMetadata)

	return buildResponsesObjectMap(
		responsesIDFromGemini(geminiResp, idBase), createdAt, "completed", modelName,
		output, buildResponsesUsageMap(usage), &completedAt, info,
	), nil
}

// buildResponsesOutputFromGemini 构建 Responses 非流式响应的 output 数组。
// 文本合并为单个 message 项（居首），functionCall 各成一个 function_call 项。
func buildResponsesOutputFromGemini(geminiResp *dto.GeminiChatResponse, idBase string) []map[string]any {
	var textParts []string
	output := make([]map[string]any, 0)
	toolIdx := 0

	for _, candidate := range geminiResp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for i := range candidate.Content.Parts {
			part := &candidate.Content.Parts[i]
			isThought := part.Thought != nil && *part.Thought

			if part.Text != "" && !isThought {
				textParts = append(textParts, part.Text)
			}
			// 思考内容无 Responses 非流式对应物，跳过（与 claude→responses 口径一致）

			if part.FunctionCall != nil {
				id := geminiResponsesCallID(part.FunctionCall, idBase, toolIdx)
				toolIdx++
				output = append(output, map[string]any{
					"type":      "function_call",
					"id":        id,
					"call_id":   id,
					"name":      part.FunctionCall.FunctionName,
					"arguments": marshalGeminiArgs(part.FunctionCall),
					"status":    "completed",
				})
			}

			if fallback := geminiPartFallbackText(part); fallback != "" {
				textParts = append(textParts, fallback)
			}
		}
	}

	if len(textParts) > 0 {
		msgItem := map[string]any{
			"type":   "message",
			"id":     fmt.Sprintf("msg_%s", idBase),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{{
				"type":        "output_text",
				"text":        strings.Join(textParts, ""),
				"annotations": []any{},
			}},
		}
		output = append([]map[string]any{msgItem}, output...)
	}

	return output
}

// responsesIDFromGemini 生成 Responses 响应 ID，优先用上游 responseId；
// idBase 为本次转换的唯一基准（宿主 RequestID 未随 Meta 下沉，以纳秒时间戳替代）
func responsesIDFromGemini(geminiResp *dto.GeminiChatResponse, idBase string) string {
	if geminiResp.ResponseID != "" {
		return fmt.Sprintf("resp_%s", geminiResp.ResponseID)
	}
	return fmt.Sprintf("resp_%s", idBase)
}

// geminiResponsesCallID 取工具调用 ID：Gemini 的 functionCall.id 可选，缺失时合成。
// Responses 的 function_call 必须带 call_id，客户端下一轮按此 ID 回传结果。
func geminiResponsesCallID(fc *dto.GeminiFunctionCall, idBase string, idx int) string {
	if fc != nil && fc.ID != "" {
		return fc.ID
	}
	return fmt.Sprintf("call_%s_%d", idBase, idx)
}

// geminiPartFallbackText 将 Responses 的 output_text 无对应块类型的 Gemini part 渲染为文本。
// 渲染口径与 Gemini→OpenAI 出站一致。
func geminiPartFallbackText(part *dto.GeminiPart) string {
	switch {
	case part.InlineData != nil:
		return fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
	case part.ExecutableCode != nil:
		return fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code)
	case part.CodeExecutionResult != nil:
		return fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output)
	case part.FileData != nil:
		return fmt.Sprintf("[file](%s)", part.FileData.FileURI)
	}
	return ""
}
