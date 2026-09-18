package oai_responses

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToResponsesResponseConverter 将 Chat Completions 非流式响应转换为 Responses API 响应
// （Responses 入站 → chat 上游方向的响应合成）。
type OpenAIToResponsesResponseConverter struct{}

func (c *OpenAIToResponsesResponseConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToOAIResponses
}

func (c *OpenAIToResponsesResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToResponsesResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *OpenAIToResponsesResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertResponse 将 Chat Completions 非流式响应转换为 Responses API 非流式响应。
func (c *OpenAIToResponsesResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	chatResp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}
	return chatCompletionToResponsesResponse(chatResp, info), nil
}

// responsesRequestEcho 合成 Responses 响应时需回显（echo）的请求参数
type responsesRequestEcho struct {
	temperature     *float64
	topP            *float64
	maxOutputTokens *int
	instructions    any
}

// extractResponsesRequestEcho 从 ResponsesStash 能力接口（宿主 stash 的请求快照）提取
// 合成响应应 echo 的请求参数。info 未实现该接口或快照缺失（直连路径/异常）时
// 回退 OpenAI 默认值（temperature=1.0 / top_p=1.0，其余 nil）。
func extractResponsesRequestEcho(info convmeta.Meta) responsesRequestEcho {
	echo := responsesRequestEcho{temperature: float64Ptr(1.0), topP: float64Ptr(1.0)}
	stash, ok := info.(convmeta.ResponsesStash)
	if !ok {
		return echo
	}
	rr := stash.StashedResponsesRequest()
	if rr == nil {
		return echo
	}
	if rr.Temperature != nil {
		echo.temperature = rr.Temperature
	}
	if rr.TopP != nil {
		echo.topP = rr.TopP
	}
	if rr.MaxOutputTokens != nil {
		m := int(*rr.MaxOutputTokens)
		echo.maxOutputTokens = &m
	}
	if len(rr.Instructions) > 0 {
		echo.instructions = json.RawMessage(rr.Instructions)
	}
	return echo
}

// chatCompletionToResponsesResponse 将 Chat Completions 响应转换为 Responses API 响应
func chatCompletionToResponsesResponse(chatResp *dto.ChatCompletionResponse, info convmeta.Meta) *dto.OpenAIResponsesResponse {
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}
	echo := extractResponsesRequestEcho(info)

	// 构建 output
	output := make([]dto.ResponsesOutput, 0)
	for _, choice := range chatResp.Choices {
		// 思考内容 → reasoning 输出项（排在 message 之前，与 Responses API 的真实
		// 输出顺序一致）。此前一律跳过，推理模型（DeepSeek-R1 / o 系列等）的思考过程
		// 经本方向转换后会静默消失。
		if choice.Message.ReasoningContent != nil {
			if item := shared.BuildResponsesReasoningOutput(chatResp.ID, *choice.Message.ReasoningContent); item != nil {
				output = append(output, *item)
			}
		}

		// 文本内容
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

		// 工具调用
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

		// 思考文本跨轮携带：按本轮工具调用 call_id 存进宿主缓存（与流式路径同口径），
		// 供客户端剥掉 reasoning 项时下一轮按 call_id 还原 reasoning_content
		if carry, ok := info.(convmeta.ReasoningCarry); ok &&
			choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" &&
			len(choice.Message.ToolCalls) > 0 {
			callIDs := make([]string, 0, len(choice.Message.ToolCalls))
			for _, tc := range choice.Message.ToolCalls {
				callIDs = append(callIDs, tc.ID)
			}
			carry.StoreReasoningForCalls(callIDs, *choice.Message.ReasoningContent)
		}
	}

	// usage 明细：上游 chat 响应可能不携带 details（nil 指针），缺失时按零值合成
	inputDetails := &dto.InputTokenDetails{}
	if d := chatResp.Usage.PromptTokensDetails; d != nil {
		inputDetails.CachedTokens = d.CachedTokens
		inputDetails.CacheWriteTokens = d.CacheWriteTokens
		inputDetails.AudioTokens = d.AudioTokens
	}
	outputDetails := &dto.OutputTokenDetails{}
	if d := chatResp.Usage.CompletionTokenDetails; d != nil {
		outputDetails.ReasoningTokens = d.ReasoningTokens
		outputDetails.AcceptedPredictionTokens = d.AcceptedPredictionTokens
		outputDetails.RejectedPredictionTokens = d.RejectedPredictionTokens
	}

	return &dto.OpenAIResponsesResponse{
		ID:                 fmt.Sprintf("resp_%s", chatResp.ID),
		Object:             "response",
		CreatedAt:          int(chatResp.Created),
		CompletedAt:        int(chatResp.Created) + 1,
		Status:             json.RawMessage(`"completed"`),
		Error:              nil,
		IncompleteDetails:  nil,
		Instructions:       echo.instructions,
		MaxOutputTokens:    echo.maxOutputTokens,
		Model:              modelName,
		Output:             output,
		ParallelToolCalls:  true,
		PreviousResponseID: nil,
		Reasoning:          &dto.ResponsesReasoning{Effort: nil, Summary: nil},
		// store:false 是真实语义：合成响应不落上游存储，客户端不可经 GET /v1/responses/{id} retrieve
		Store:       false,
		Temperature: echo.temperature,
		Text:        &dto.ResponsesText{Format: dto.ResponsesTextFormat{Type: "text"}},
		ToolChoice:  "auto",
		Tools:       make([]any, 0),
		TopP:        echo.topP,
		Truncation:  "disabled",
		User:        nil,
		Metadata:    make(map[string]any),
		Usage: &dto.ResponsesUsage{
			InputTokens:        chatResp.Usage.PromptTokens,
			OutputTokens:       chatResp.Usage.CompletionTokens,
			TotalTokens:        chatResp.Usage.TotalTokens,
			InputTokensDetails: inputDetails,
			OutputTokenDetails: outputDetails,
		},
	}
}

// float64Ptr 返回 float64 的指针
func float64Ptr(v float64) *float64 {
	return &v
}
