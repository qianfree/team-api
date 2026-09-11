package oai_responses

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIResponseConverter 将 Responses API 非流式响应转换为 Chat Completions 响应。
type ResponsesToOpenAIResponseConverter struct{}

func (c *ResponsesToOpenAIResponseConverter) ID() string {
	return relayconvert.ResponseConverterOAIResponsesToOAIChat
}

func (c *ResponsesToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ResponsesToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 Responses API 非流式响应转换为 Chat Completions 非流式响应。
// usage 信息写入 ChatCompletionResponse.Usage 字段（宿主由此提取计费用量）。
func (c *ResponsesToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	resp, ok := response.(*dto.OpenAIResponsesResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.OpenAIResponsesResponse, got %T", response)
	}

	// 模型名：映射渠道回写客户端请求的模型名；未映射时优先用上游响应携带的模型名
	modelName := resp.Model
	if modelName == "" {
		modelName = convmeta.UpstreamModelName(info)
	}
	if isModelMapped(info) || modelName == "" {
		if info != nil {
			modelName = info.GetOriginModelName()
		}
	}

	chatID := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	return responsesResponseToChatCompletions(resp, chatID, modelName), nil
}

// responsesResponseToChatCompletions 将 Responses API 非流式响应转换为 Chat Completions 响应
// （移植宿主 ResponsesResponseToChatCompletions 的纯转换部分，usage 并入返回值的 Usage 字段）。
func responsesResponseToChatCompletions(resp *dto.OpenAIResponsesResponse, id string, model string) *dto.ChatCompletionResponse {
	text, toolCalls := extractOutputFromResponses(resp)

	finishReason := "stop"
	content := ""
	if len(toolCalls) > 0 && text == "" {
		finishReason = "tool_calls"
	}
	if text != "" || len(toolCalls) == 0 {
		content = text
	}

	usage := &dto.UsageWithDetails{}
	if resp.Usage != nil {
		usage.PromptTokens = resp.Usage.InputTokens
		usage.CompletionTokens = resp.Usage.OutputTokens
		usage.TotalTokens = resp.Usage.TotalTokens
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails = &dto.TokenDetails{
				CachedTokens:     resp.Usage.InputTokensDetails.CachedTokens,
				CacheWriteTokens: resp.Usage.InputTokensDetails.CacheWriteTokens,
				AudioTokens:      resp.Usage.InputTokensDetails.AudioTokens,
				TextTokens:       resp.Usage.InputTokensDetails.TextTokens,
				ImageTokens:      resp.Usage.InputTokensDetails.ImageTokens,
			}
		}
		if resp.Usage.OutputTokenDetails != nil {
			usage.CompletionTokenDetails = &dto.TokenDetails{
				ReasoningTokens:          resp.Usage.OutputTokenDetails.ReasoningTokens,
				AudioTokens:              resp.Usage.OutputTokenDetails.AudioTokens,
				TextTokens:               resp.Usage.OutputTokenDetails.TextTokens,
				AcceptedPredictionTokens: resp.Usage.OutputTokenDetails.AcceptedPredictionTokens,
				RejectedPredictionTokens: resp.Usage.OutputTokenDetails.RejectedPredictionTokens,
			}
		}
	} else if text != "" {
		// 上游未返回 usage 时按文本长度估算（4 字符/token）
		estimated := len(text) / 4
		usage.CompletionTokens = estimated
		usage.TotalTokens = usage.PromptTokens + estimated
	}

	// reasoning 输出项 → chat 的 reasoning_content。此前一律丢弃，
	// 推理模型经本方向转换后客户端看不到任何思考过程。
	msg := dto.Message{Role: "assistant", Content: content, ToolCalls: toolCalls}
	if thinking := shared.ExtractResponsesReasoning(resp.Output); thinking != "" {
		msg.ReasoningContent = &thinking
	}

	return &dto.ChatCompletionResponse{
		ID: id, Object: "chat.completion", Created: time.Now().Unix(), Model: model,
		Choices: []dto.Choice{{
			Index: 0, Message: msg, FinishReason: finishReason,
		}},
		Usage: *usage,
	}
}

func extractOutputFromResponses(resp *dto.OpenAIResponsesResponse) (string, []dto.ToolCall) {
	var textParts []string
	var toolCalls []dto.ToolCall
	for _, output := range resp.Output {
		switch output.Type {
		case "message":
			if output.Role == "assistant" {
				for _, c := range output.Content {
					if c.Type == "output_text" && c.Text != "" {
						textParts = append(textParts, c.Text)
					}
				}
			}
		case "function_call":
			toolCalls = append(toolCalls, dto.ToolCall{
				ID: output.CallID, Type: "function",
				Function: dto.FunctionCall{Name: output.Name, Arguments: output.Arguments},
			})
		}
	}
	return strings.Join(textParts, ""), toolCalls
}

// isModelMapped 判断渠道是否配置了模型映射（上游模型名 ≠ 客户端请求模型名）。
// Meta 未暴露宿主的 IsModelMapped 标记，以「已附加渠道信息且上游名与原始名不同」等价判定。
func isModelMapped(info convmeta.Meta) bool {
	if info == nil || !info.HasChannelMeta() {
		return false
	}
	upstream := info.GetUpstreamModelName()
	return upstream != "" && upstream != info.GetOriginModelName()
}
