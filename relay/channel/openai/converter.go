package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ConvertToOpenAI 根据入站格式将请求转换为 OpenAI 格式。
// 如果入站已是 OpenAI 格式（或空），原样返回。
// 其他供应商适配器可调用此函数统一处理入站格式预转换。
//
// claude/gemini/responses 入站：relaykit 唯一路径（P1-A 接管点在此函数内部而非 handler
// 桥接——20+ 个 openai 兼容 adaptor 在本函数之后各有定制后处理，接管点在内部可保持后处理
// 不变。responses 入站对 ollama/coze/dify 等原生格式上游也在此转换——handler 桥的 responses
// 路由只认 openai/claude/gemini 上游）。legacy 回退转换器已收割：转换失败直接报错，
// 问题经 monitor.TrackConverterCall 显式暴露。
func ConvertToOpenAI(requestBody []byte, info *common.RelayInfo) ([]byte, error) {
	switch info.InboundFormat {
	case constant.RelayFormatClaude, constant.RelayFormatGemini, constant.RelayFormatResponses:
		if out, ok := relaykit_bridge.TryConvertInboundToOpenAIChat(context.Background(), info, requestBody); ok {
			return out, nil
		}
		return nil, fmt.Errorf("[relaykit] %s→openai 请求转换失败（无匹配转换器或转换出错）", info.InboundFormat)
	default:
		return requestBody, nil
	}
}

// HandleResponsesStreamToChat 将 Responses API 流式响应实时转换为 Chat Completions SSE 流
func HandleResponsesStreamToChat(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体）。必须在 StreamScannerHandler 写出
	// 200 SSE 响应头之前拦截，否则错误体会被当作 SSE 流转发、且无法再向客户端传递状态码。
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		}
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	responseID := fmt.Sprintf("chatcmpl-%s", info.RequestID)
	createAt := time.Now().Unix()
	model := info.ChannelMeta.UpstreamModelName

	var (
		totalUsage            common.Usage
		usageText, outputText strings.Builder
		sentStart, sentStop   bool
		sawToolCall           bool
	)

	toolCallIndexByID := make(map[string]int)
	toolCallNameByID := make(map[string]string)
	toolCallArgsByID := make(map[string]string)
	toolCallNameSent := make(map[string]bool)
	// item.id → call_id 归一：function_call_arguments.delta 只携带 item_id（output item 的
	// id，如 "fc_xxx"，≠ call_id "call_xxx"），不归一到同一键时 delta 落新键 → tool_calls
	// 碎裂成两个 index，done 再全量重发参数 → 客户端组装非法 JSON 工具入参
	//（与 relaykit 侧 responses_to_oai_chat_stream 的同款修复保持同步）
	callIDByItemID := make(map[string]string)

	sendChatChunk := func(chunk *dto.ChatCompletionStreamResponse) bool {
		if chunk == nil {
			return true
		}
		data, err := json.Marshal(chunk)
		if err != nil {
			return false
		}
		return helper.WriteSSEData(writer, string(data)) == nil
	}

	sendStartIfNeeded := func() bool {
		if sentStart {
			return true
		}
		chunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Role: "assistant", Content: ""}}},
		}
		if !sendChatChunk(chunk) {
			return false
		}
		sentStart = true
		return true
	}

	helper.StreamScannerHandler(ctx, resp, info, writer, func(data string, sr *helper.StreamResult) {
		var streamResp dto.ResponsesStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			sr.Error(fmt.Errorf("unmarshal responses stream: %w", err))
			return
		}

		switch streamResp.Type {
		case "response.created":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
			}

		case "response.reasoning_summary_text.delta":
			if streamResp.Delta == "" {
				return
			}
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			usageText.WriteString(streamResp.Delta)
			delta := streamResp.Delta
			chunk := &dto.ChatCompletionStreamResponse{
				ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
				Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ReasoningContent: &delta}}},
			}
			if !sendChatChunk(chunk) {
				sr.Stop(fmt.Errorf("send reasoning chunk failed"))
				return
			}

		case "response.output_text.delta":
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			if streamResp.Delta != "" {
				outputText.WriteString(streamResp.Delta)
				usageText.WriteString(streamResp.Delta)
				delta := streamResp.Delta
				chunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{Content: delta}}},
				}
				if !sendChatChunk(chunk) {
					sr.Stop(fmt.Errorf("send text chunk failed"))
					return
				}
			}

		case "response.output_item.added", "response.output_item.done":
			if streamResp.Item == nil || streamResp.Item.Type != "function_call" {
				return
			}
			callID := strings.TrimSpace(streamResp.Item.CallID)
			if callID == "" {
				callID = strings.TrimSpace(streamResp.Item.ID)
			}
			if callID == "" {
				return
			}
			// 登记 item.id → call_id，供 delta 事件归一键（见 callIDByItemID 声明处注释）
			if itemID := strings.TrimSpace(streamResp.Item.ID); itemID != "" && itemID != callID {
				callIDByItemID[itemID] = callID
			}
			name := strings.TrimSpace(streamResp.Item.Name)
			if name != "" {
				toolCallNameByID[callID] = name
			}
			newArgs := streamResp.Item.Arguments
			prevArgs := toolCallArgsByID[callID]
			var argsDelta string
			if newArgs != "" {
				if strings.HasPrefix(newArgs, prevArgs) {
					argsDelta = newArgs[len(prevArgs):]
				} else {
					argsDelta = newArgs
				}
				toolCallArgsByID[callID] = newArgs
			}
			if !r2cSendToolCallChunk(responseID, createAt, model, callID, name, argsDelta, toolCallIndexByID, toolCallNameByID, toolCallNameSent, writer, sendChatChunk) {
				sr.Stop(fmt.Errorf("send tool call chunk failed"))
				return
			}
			sawToolCall = true
			usageText.WriteString(name)
			usageText.WriteString(argsDelta)

		case "response.function_call_arguments.delta":
			itemID := strings.TrimSpace(streamResp.ItemID)
			// 归一到 output_item 事件的 call_id 键（见 callIDByItemID 声明处注释）
			callID := itemID
			if mapped, ok := callIDByItemID[itemID]; ok {
				callID = mapped
			}
			if callID == "" {
				return
			}
			toolCallArgsByID[callID] += streamResp.Delta
			if !r2cSendToolCallChunk(responseID, createAt, model, callID, "", streamResp.Delta, toolCallIndexByID, toolCallNameByID, toolCallNameSent, writer, sendChatChunk) {
				sr.Stop(fmt.Errorf("send tool call args chunk failed"))
				return
			}
			sawToolCall = true
			usageText.WriteString(streamResp.Delta)

		case "response.completed":
			if streamResp.Response != nil {
				if streamResp.Response.Model != "" {
					model = streamResp.Response.Model
				}
				if streamResp.Response.CreatedAt != 0 {
					createAt = int64(streamResp.Response.CreatedAt)
				}
				if streamResp.Response.Usage != nil {
					totalUsage = *responsesUsageToCommon(streamResp.Response.Usage)
					if totalUsage.TotalTokens == 0 {
						totalUsage.TotalTokens = totalUsage.PromptTokens + totalUsage.CompletionTokens
					}
				}
			}
			if !sendStartIfNeeded() {
				sr.Stop(fmt.Errorf("send start chunk failed"))
				return
			}
			if !sentStop {
				finishReason := "stop"
				if sawToolCall && outputText.Len() == 0 {
					finishReason = "tool_calls"
				}
				fr := finishReason
				stopChunk := &dto.ChatCompletionStreamResponse{
					ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
					Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
				}
				if !sendChatChunk(stopChunk) {
					sr.Stop(fmt.Errorf("send stop chunk failed"))
					return
				}
				sentStop = true
			}

		case "response.error", "response.failed":
			errMsg := "responses stream error"
			if streamResp.Response != nil && streamResp.Response.Error != nil {
				if b, err := json.Marshal(streamResp.Response.Error); err == nil {
					errMsg = string(b)
				}
			}
			sr.Stop(fmt.Errorf("%s: %s", streamResp.Type, errMsg))
			return
		}
	})

	// 上游未返回 usage 时按累计文本估算（正常结束 4 字符/token，中断 2 字符/token）
	if totalUsage.TotalTokens == 0 && usageText.Len() > 0 {
		estimated := helper.EstimateStreamOutputTokens(info, usageText.Len())
		totalUsage.CompletionTokens = estimated
		totalUsage.TotalTokens = totalUsage.PromptTokens + estimated
	}
	// 流中断：输入 token 用请求侧估算值补齐，保证输入正常计费
	helper.ApplyInterruptedUsageFallback(info, &totalUsage, usageText.Len())
	if !sentStart {
		sendStartIfNeeded()
	}
	if !sentStop {
		fr := "stop"
		stopChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{{Index: 0, FinishReason: &fr}},
		}
		sendChatChunk(stopChunk)
	}
	if totalUsage.TotalTokens > 0 {
		usageChunk := &dto.ChatCompletionStreamResponse{
			ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
			Choices: []dto.StreamChoice{},
			Usage:   &dto.UsageWithDetails{PromptTokens: totalUsage.PromptTokens, CompletionTokens: totalUsage.CompletionTokens, TotalTokens: totalUsage.TotalTokens, PromptTokensDetails: common.CommonTokenDetailsToDto(totalUsage.PromptTokensDetails), CompletionTokenDetails: common.CommonTokenDetailsToDto(totalUsage.CompletionTokenDetails)},
		}
		sendChatChunk(usageChunk)
	}
	helper.WriteSSEData(writer, "[DONE]")

	g.Log().Infof(ctx, "[HandleResponsesStreamToChat] completed: usage=%+v, endReason=%s", totalUsage, info.StreamStatus.GetEndReason())
	return &totalUsage, nil
}

func r2cSendToolCallChunk(responseID string, createAt int64, model string, callID string, name string, argsDelta string, indexByID map[string]int, nameByID map[string]string, nameSent map[string]bool, writer http.ResponseWriter, sendChatChunk func(*dto.ChatCompletionStreamResponse) bool) bool {
	idx, ok := indexByID[callID]
	if !ok {
		idx = len(indexByID)
		indexByID[callID] = idx
	}
	if name != "" {
		nameByID[callID] = name
	}
	if nameByID[callID] != "" {
		name = nameByID[callID]
	}
	tool := dto.ToolCall{ID: callID, Type: "function", Index: idx, Function: dto.FunctionCall{Arguments: argsDelta}}
	if name != "" && !nameSent[callID] {
		tool.Function.Name = name
		nameSent[callID] = true
	}
	chunk := &dto.ChatCompletionStreamResponse{
		ID: responseID, Object: "chat.completion.chunk", Created: createAt, Model: model,
		Choices: []dto.StreamChoice{{Index: 0, Delta: dto.Message{ToolCalls: []dto.ToolCall{tool}}}},
	}
	return sendChatChunk(chunk)
}

// HandleResponsesNonStreamToChat 将 Responses API 非流式响应转换为 Chat Completions 并写入 writer。
// relaykit 唯一路径（openai 客户端 × UseResponsesAPI 渠道的非流式响应）；失败直接报错。
func HandleResponsesNonStreamToChat(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		// 错误体已写给客户端，标记 ResponseWritten 防止调度 FSM 误判为可重试导致二次写入
		upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
		upstreamErr.ResponseWritten = true
		return nil, upstreamErr
	}

	chatBody, usage, ok := relaykit_bridge.TryConvertChatViaResponsesResponseViaRelaykit(ctx, info, body)
	if !ok {
		return nil, fmt.Errorf("[relaykit] responses→chat 非流式响应转换失败")
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(chatBody)
	return usage, nil
}
