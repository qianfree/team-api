package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIChatToResponsesResponseConverter OpenAI Chat 上游 → Responses 客户端（非流式响应侧）。
// 移植自宿主 relay/channel/openai/responses.go 的 chatCompletionToResponsesResponse，
// 挂在 r2c spec（ConverterOpenAIResponsesToOpenAIChat）的 Resp 侧（方向与请求相反）。
// 与 legacy 的确定性行为差异（顺手修复项）：
//   - CompletedAt = CreatedAt（legacy 为 Created+1 的拍脑袋值）；
//   - Created 为 0 时用 NowFunc 兜底（legacy 会输出 created_at:0）；
//   - usage details 为 nil 指针时安全输出零值（legacy 直接解引用，上游不带
//     prompt_tokens_details 时会 panic——隐性 bug 修复）。
type OpenAIChatToResponsesResponseConverter struct{}

func (c *OpenAIChatToResponsesResponseConverter) ID() string {
	return relayconvert.ConverterOpenAIResponsesToOpenAIChat
}

func (c *OpenAIChatToResponsesResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIChatToResponsesResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

// ConvertResponse 入参断言 *dto.ChatCompletionResponse，输出 *dto.OpenAIResponsesResponse。
func (c *OpenAIChatToResponsesResponseConverter) ConvertResponse(
	ctx context.Context, info convmeta.Meta, response any,
) (any, *dto.UsageWithDetails, error) {
	chatResp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}
	result, usage := chatCompletionToResponses(info, chatResp)
	return result, usage, nil
}

// outputDetailOf nil 安全地提取 output 细分字段。
func outputDetailOf(usage *dto.UsageWithDetails, pick func(*dto.TokenDetails) int) int {
	if usage == nil || usage.CompletionTokenDetails == nil {
		return 0
	}
	return pick(usage.CompletionTokenDetails)
}

// chatCompletionToResponses 将 chat 非流式响应转换为 Responses 响应对象。
// 模型名恒取客户端请求模型名（legacy 精确口径，与 Claude 方向不同）。
// 返回的 usage 为客户端可见口径（OpenAI 语义，含 details）。
func chatCompletionToResponses(info convmeta.Meta, chatResp *dto.ChatCompletionResponse) (*dto.OpenAIResponsesResponse, *dto.UsageWithDetails) {
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}
	echo := responsesEchoOf(info)

	// 构建 output：每 choice 依次产出 reasoning 项（有思考内容时，先于 message——真实
	// OpenAI 项序，codex 等客户端据此在后续轮次回传思考内容）、message 项（仅内容非空时保留：
	// DTO 的 content 为 omitempty，空切片会丢掉整个 content 键，OpenAI SDK 等严格客户端把
	// message.content 视为必填会解析失败；真实 OpenAI 的工具调用响应也不含空 message 项，
	// 与流式侧 completed output 的保留口径一致）、tool_call 项
	output := make([]dto.ResponsesOutput, 0)
	for i, choice := range chatResp.Choices {
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			rsID := fmt.Sprintf("rs_%s", chatResp.ID)
			if i > 0 {
				rsID = fmt.Sprintf("rs_%s_%d", chatResp.ID, i)
			}
			output = append(output, dto.ResponsesOutput{
				Type: "reasoning",
				ID:   rsID,
				Summary: []dto.ResponsesSummaryPart{{
					Type: "summary_text",
					Text: *choice.Message.ReasoningContent,
				}},
			})
		}

		content := make([]dto.ResponsesOutputContent, 0)
		if choice.Message.Content != nil {
			var textContent string
			switch v := choice.Message.Content.(type) {
			case string:
				textContent = v
			default:
				b, _ := json.Marshal(v)
				textContent = string(b)
			}
			if textContent != "" {
				content = append(content, dto.ResponsesOutputContent{
					Type:        "output_text",
					Text:        textContent,
					Annotations: []dto.ResponsesAnnotation{},
				})
			}
		}

		if len(content) > 0 {
			output = append(output, dto.ResponsesOutput{
				Type:    "message",
				ID:      fmt.Sprintf("msg_%s", chatResp.ID),
				Status:  "completed",
				Role:    "assistant",
				Content: content,
			})
		}

		for _, tc := range choice.Message.ToolCalls {
			// 按请求侧 stash 的原始工具类型还原输出项（custom_tool_call /
			// local_shell_call / apply_patch_call），未 stash 为 function_call
			output = append(output, buildToolCallDoneItem(info, tc.ID, tc.Function.Name, tc.Function.Arguments))
		}
	}

	createdAt := int(chatResp.Created)
	if createdAt == 0 {
		createdAt = int(NowFunc().Unix())
	}
	completedAt := createdAt

	usage := &dto.UsageWithDetails{
		PromptTokens:     chatResp.Usage.PromptTokens,
		CompletionTokens: chatResp.Usage.CompletionTokens,
		TotalTokens:      chatResp.Usage.TotalTokens,
	}
	if chatResp.Usage.PromptTokensDetails != nil {
		usage.PromptTokensDetails = chatResp.Usage.PromptTokensDetails
	}
	if chatResp.Usage.CompletionTokenDetails != nil {
		usage.CompletionTokenDetails = chatResp.Usage.CompletionTokenDetails
	}
	usageObj := responsesUsageOf(usage)
	// chat 方向 output 细分不拷 audio_tokens（legacy chatCompletionToResponsesResponse 口径，
	// 与 claude 方向 BuildResponsesUsageMap 拷贝 audio 的口径不同）；键恒存在由 DTO tag 保证
	usageObj.OutputTokenDetails = &dto.OutputTokenDetails{
		ReasoningTokens:          outputDetailOf(usage, func(d *dto.TokenDetails) int { return d.ReasoningTokens }),
		AcceptedPredictionTokens: outputDetailOf(usage, func(d *dto.TokenDetails) int { return d.AcceptedPredictionTokens }),
		RejectedPredictionTokens: outputDetailOf(usage, func(d *dto.TokenDetails) int { return d.RejectedPredictionTokens }),
	}

	return buildResponsesObject(fmt.Sprintf("resp_%s", chatResp.ID), createdAt, "completed",
		modelName, output, usageObj, &completedAt, echo), usage
}
