package oai_responses

import (
	"context"
	"fmt"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// ResponsesToOpenAIChatResponseConverter Responses 上游 → OpenAI Chat 客户端（非流式响应侧）。
// 移植自宿主 relay/channel/openai/converter.go 的 ResponsesResponseToChatCompletions +
// HandleResponsesNonStreamToChat 的模型名三段逻辑，挂在 c2r spec 的 Resp 侧
//（ChatViaResponses 渠道：chat 客户端经 Responses API 桥接）。
// legacy 语义保持项（golden 13 锁定）：
//   - 文本+工具并存时 content 保留文本、finish_reason=stop（部分按 finish_reason 分派的
//     客户端会忽略工具调用——legacy 取舍，流式侧同理）；
//   - 上游 usage 缺失且有文本时按 len(text)/4 硬编码估算。
type ResponsesToOpenAIChatResponseConverter struct{}

func (c *ResponsesToOpenAIChatResponseConverter) ID() string {
	return relayconvert.ConverterOpenAIChatToOpenAIResponses
}

func (c *ResponsesToOpenAIChatResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ResponsesToOpenAIChatResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

// ConvertResponse 入参断言 *dto.OpenAIResponsesResponse，输出 *dto.ChatCompletionResponse。
func (c *ResponsesToOpenAIChatResponseConverter) ConvertResponse(
	ctx context.Context, info convmeta.Meta, response any,
) (any, *dto.UsageWithDetails, error) {
	resp, ok := response.(*dto.OpenAIResponsesResponse)
	if !ok {
		return nil, nil, fmt.Errorf("expected *dto.OpenAIResponsesResponse, got %T", response)
	}
	result, usage := responsesToChatCompletion(info, resp)
	return result, usage, nil
}

// responsesToChatCompletion 将 Responses 非流式响应转换为 chat 响应。
func responsesToChatCompletion(info convmeta.Meta, resp *dto.OpenAIResponsesResponse) (*dto.ChatCompletionResponse, *dto.UsageWithDetails) {
	text, toolCalls := extractOutputFromResponses(resp)

	finishReason := "stop"
	content := ""
	if len(toolCalls) > 0 && text == "" {
		finishReason = "tool_calls"
	}
	// legacy 语义：text 非空或无工具时 content=text（文本+工具并存时文本保留、finish=stop）
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
				CachedTokens: resp.Usage.InputTokensDetails.CachedTokens,
				AudioTokens:  resp.Usage.InputTokensDetails.AudioTokens,
				TextTokens:   resp.Usage.InputTokensDetails.TextTokens,
				ImageTokens:  resp.Usage.InputTokensDetails.ImageTokens,
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
		estimated := len(text) / 4
		usage.CompletionTokens = estimated
		usage.TotalTokens = usage.PromptTokens + estimated
	}

	// 模型名三段（legacy 精确口径）：映射渠道 → 客户端请求模型名；
	// 未映射 → 上游返回值，缺失回退上游映射名
	model := ""
	if info != nil {
		if modelMappedOf(info) {
			model = info.GetOriginModelName()
		} else if resp.Model != "" {
			model = resp.Model
		} else {
			model = info.GetUpstreamModelName()
		}
	} else if resp.Model != "" {
		model = resp.Model
	}

	// ID：请求 ID 合成（缺失时用 NowFunc 兜底，golden 冻结时钟下确定）
	requestID := requestIDOf(info)
	if requestID == "" {
		requestID = fmt.Sprintf("%d", NowFunc().UnixNano())
	}

	chatResp := &dto.ChatCompletionResponse{
		ID: fmt.Sprintf("chatcmpl-%s", requestID), Object: "chat.completion",
		Created: NowFunc().Unix(), Model: model,
		Choices: []dto.Choice{{
			Index: 0, Message: dto.Message{Role: "assistant", Content: content, ToolCalls: toolCalls},
			FinishReason: finishReason,
		}},
		Usage: *usage,
	}
	return chatResp, usage
}

// extractOutputFromResponses 提取 message 项的 output_text（无分隔符拼接）与 function_call 项；
// 其它输出项类型（web_search_call/reasoning 等）无 chat 对应物，丢弃。
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
