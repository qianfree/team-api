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

	// 构建 output：每 choice 一个 message 项（空内容也保留，content 为显式空切片
	// 序列化成 [] 而非 null——codex 等严格客户端要求），随后每个 tool_call 一项
	output := make([]dto.ResponsesOutput, 0)
	for _, choice := range chatResp.Choices {
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

		msgOutput := dto.ResponsesOutput{
			Type:    "message",
			ID:      fmt.Sprintf("msg_%s", chatResp.ID),
			Status:  "completed",
			Role:    "assistant",
			Content: content,
		}
		output = append(output, msgOutput)

		for _, tc := range choice.Message.ToolCalls {
			output = append(output, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        tc.ID,
				CallID:    tc.ID,
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
				Status:    "completed",
			})
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
