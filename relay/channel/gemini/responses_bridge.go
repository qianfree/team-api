package gemini

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// ========== Responses 入站桥接：Gemini → OpenAI Responses ==========
//
// 请求侧由 ConvertResponsesToGemini 完成（Responses → OpenAI → Gemini），
// 这里做响应侧：把 Gemini 上游的响应/SSE 转成 Responses 格式。
// 事件发射器复用 openai 包（EmitResponsesSSE / BuildResponsesObjectMap / BuildResponsesUsageMap），
// 与 chat→responses（openai/responses.go）、claude→responses（claude/responses_bridge.go）
// 两条既有桥接的事件序列保持一致。
//
// 用量口径：Responses 的客户端可见 usage 与本渠道的计费口径**恰好一致**
// （都是 OpenAI 语义：input 含缓存、output 含思考），故两者共用 geminiUsageToCommon，
// 不像 claude→responses 那样需要两套换算。
//
// 协议落差：
//   - Gemini 的 functionCall 未必带 id，Responses 的 function_call 必须有 call_id → 缺失时合成。
//   - 思考内容在 Responses 非流式响应里没有对应物（沿用 claude→responses 的口径跳过），
//     流式侧以 response.reasoning_summary_text.delta 透出。

// geminiResponsesToolCall Responses 桥接中聚合的工具调用（Gemini functionCall）
type geminiResponsesToolCall struct {
	id   string
	name string
	args string
}

// handleNonStreamToResponses 将 Gemini 非流式响应转换为 Responses 格式
func (a *Adaptor) handleNonStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}
	if resp.StatusCode != http.StatusOK {
		// 不写响应：交上层 WriteRelayError 统一写入，避免双重写入与重试时的响应污染
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	var geminiResp dto.GeminiChatResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err)
	}

	if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
		return nil, constant.NewRequestError(
			fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
		)
	}

	responsesBody, usage, err := buildResponsesBodyFromGemini(&geminiResp, info)
	if err != nil {
		return nil, err
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responsesBody)

	return usage, nil
}

// buildResponsesBodyFromGemini 构建 Responses 非流式响应体与用量。
// 直连非流式与 Code Assist 强制流式的聚合结果共用同一套装配逻辑。
func buildResponsesBodyFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) ([]byte, *common.Usage, error) {
	output := buildResponsesOutputFromGemini(geminiResp, info)

	modelName := geminiResp.ModelName
	if modelName == "" || info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}
	createdAt := int(time.Now().Unix())
	completedAt := createdAt

	usage := geminiUsageToCommon(geminiResp.UsageMetadata)

	body, err := json.Marshal(openai.BuildResponsesObjectMap(
		responsesIDFromGemini(geminiResp, info), createdAt, "completed", modelName,
		output, openai.BuildResponsesUsageMap(usage), &completedAt, info,
	))
	if err != nil {
		return nil, nil, fmt.Errorf("marshal responses body failed: %w", err)
	}
	return body, usage, nil
}

// handleStreamToResponses 将 Gemini 流式响应转换为 Responses 格式的 SSE
func (a *Adaptor) handleStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 不写响应：交上层 WriteRelayError 统一写入
		return nil, buildGeminiUpstreamError(body, resp.StatusCode)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	isCA := a.isCodeAssistActive()
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	respID := fmt.Sprintf("resp_%s", info.RequestID)
	msgID := fmt.Sprintf("msg_%s", info.RequestID)
	createdAt := int(time.Now().Unix())
	modelName := info.OriginModelName

	var (
		totalUsage    dto.GeminiUsageMetadata
		textBuf       strings.Builder
		sentCreated   bool
		sentTextDone  bool
		sentCompleted bool
		outputIndex   int
		contentIndex  int
		toolIdx       int
		toolCalls     []*geminiResponsesToolCall
		toolIndexByID = make(map[string]int)
	)

	// billingUsage 计费口径（与客户端可见口径一致），输出缺失时按已转发文本估算
	billingUsage := func() *common.Usage {
		u := geminiUsageToCommon(&totalUsage)
		if u.CompletionTokens == 0 && textBuf.Len() > 0 {
			u.CompletionTokens = helper.EstimateStreamOutputTokens(info, textBuf.Len())
			u.TotalTokens = u.PromptTokens + u.CompletionTokens
		}
		return u
	}

	// ensureCreated 发出 response.created 与首个 message 项的开场事件
	ensureCreated := func() {
		if sentCreated {
			return
		}
		openai.EmitResponsesSSE(writer, "response.created", map[string]any{
			"type":     "response.created",
			"response": openai.BuildResponsesObjectMap(respID, createdAt, "in_progress", modelName, []any{}, nil, nil, info),
		})
		openai.EmitResponsesSSE(writer, "response.output_item.added", map[string]any{
			"type":         "response.output_item.added",
			"output_index": outputIndex,
			"item": map[string]any{
				"type":    "message",
				"id":      msgID,
				"status":  "in_progress",
				"role":    "assistant",
				"content": []any{},
			},
		})
		openai.EmitResponsesSSE(writer, "response.content_part.added", map[string]any{
			"type":          "response.content_part.added",
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        "",
				"annotations": []any{},
			},
		})
		sentCreated = true
	}

	// closeTextPart 关闭文本 content part（进入工具调用或流结束时调用）
	closeTextPart := func() {
		if sentTextDone {
			return
		}
		finishedText := textBuf.String()
		openai.EmitResponsesSSE(writer, "response.output_text.done", map[string]any{
			"type":          "response.output_text.done",
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"text":          finishedText,
		})
		openai.EmitResponsesSSE(writer, "response.content_part.done", map[string]any{
			"type":          "response.content_part.done",
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        finishedText,
				"annotations": []any{},
			},
		})
		openai.EmitResponsesSSE(writer, "response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": outputIndex,
			"item": map[string]any{
				"type":   "message",
				"id":     msgID,
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{{
					"type":        "output_text",
					"text":        finishedText,
					"annotations": []any{},
				}},
			},
		})
		sentTextDone = true
		outputIndex++
	}

	// finish 发送各工具调用的收尾事件与 response.completed
	finish := func() {
		if sentCompleted {
			return
		}
		ensureCreated()
		closeTextPart()

		finalOutput := make([]map[string]any, 0)
		if textBuf.Len() > 0 {
			finalOutput = append(finalOutput, map[string]any{
				"type":   "message",
				"id":     msgID,
				"status": "completed",
				"role":   "assistant",
				"content": []map[string]any{{
					"type":        "output_text",
					"text":        textBuf.String(),
					"annotations": []any{},
				}},
			})
		}

		for _, tc := range toolCalls {
			openai.EmitResponsesSSE(writer, "response.function_call_arguments.done", map[string]any{
				"type":         "response.function_call_arguments.done",
				"item_id":      tc.id,
				"output_index": toolIndexByID[tc.id],
				"arguments":    tc.args,
			})
			openai.EmitResponsesSSE(writer, "response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": toolIndexByID[tc.id],
				"item": map[string]any{
					"type":      "function_call",
					"id":        tc.id,
					"call_id":   tc.id,
					"name":      tc.name,
					"arguments": tc.args,
					"status":    "completed",
				},
			})
			finalOutput = append(finalOutput, map[string]any{
				"type":      "function_call",
				"id":        tc.id,
				"call_id":   tc.id,
				"name":      tc.name,
				"arguments": tc.args,
				"status":    "completed",
			})
		}

		completedAt := int(time.Now().Unix())
		openai.EmitResponsesSSE(writer, "response.completed", map[string]any{
			"type": "response.completed",
			"response": openai.BuildResponsesObjectMap(respID, createdAt, "completed", modelName,
				finalOutput, openai.BuildResponsesUsageMap(billingUsage()), &completedAt, info),
		})
		sentCompleted = true

		if info.StreamStatus.GetEndReason() == "" {
			info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断：Gemini 每个 chunk 携带累计 usage，缺失部分按已转发文本与请求侧估算补齐
			interruptedUsage := geminiUsageToCommon(&totalUsage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, textBuf.Len())
			return interruptedUsage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data, _ := helper.ExtractSSEData(line)
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}
		info.SetFirstResponseTime()

		rawData := []byte(data)
		if isCA {
			rawData = unwrapCodeAssistData(rawData)
		}
		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal(rawData, &geminiResp); err != nil {
			continue
		}

		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}
		if geminiResp.ModelName != "" && !info.ChannelMeta.IsModelMapped {
			modelName = geminiResp.ModelName
		}

		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			// SSE 头已发送，必须补齐 completed 事件，否则客户端挂起
			finish()
			return billingUsage(), constant.NewRequestError(
				fmt.Sprintf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason), nil,
			)
		}

		ensureCreated()

		for _, candidate := range geminiResp.Candidates {
			if candidate.Content == nil {
				continue
			}
			for i := range candidate.Content.Parts {
				part := &candidate.Content.Parts[i]
				isThought := part.Thought != nil && *part.Thought

				if part.Text != "" {
					if isThought {
						// 思考内容以 reasoning summary 事件透出（与 claude→responses 口径一致）
						openai.EmitResponsesSSE(writer, "response.reasoning_summary_text.delta", map[string]any{
							"type":          "response.reasoning_summary_text.delta",
							"item_id":       msgID,
							"output_index":  0,
							"summary_index": 0,
							"delta":         part.Text,
						})
					} else {
						textBuf.WriteString(part.Text)
						openai.EmitResponsesSSE(writer, "response.output_text.delta", map[string]any{
							"type":          "response.output_text.delta",
							"item_id":       msgID,
							"output_index":  0,
							"content_index": contentIndex,
							"delta":         part.Text,
						})
					}
				}

				if part.FunctionCall != nil {
					// 先关闭文本 content part，再开 function_call 项
					closeTextPart()

					tc := &geminiResponsesToolCall{
						id:   geminiResponsesCallID(part.FunctionCall, info, toolIdx),
						name: part.FunctionCall.FunctionName,
						args: marshalGeminiArgs(part.FunctionCall),
					}
					toolIdx++
					toolCalls = append(toolCalls, tc)
					toolIndexByID[tc.id] = outputIndex

					openai.EmitResponsesSSE(writer, "response.output_item.added", map[string]any{
						"type":         "response.output_item.added",
						"output_index": outputIndex,
						"item": map[string]any{
							"type":    "function_call",
							"id":      tc.id,
							"call_id": tc.id,
							"name":    tc.name,
							"status":  "in_progress",
						},
					})
					// Gemini 一次给全量参数，单条 delta 发完（收尾的 .done 在 finish 中统一发）
					openai.EmitResponsesSSE(writer, "response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      tc.id,
						"output_index": outputIndex,
						"delta":        tc.args,
					})
					outputIndex++
				}

				// Responses 的 output_text 无对应块类型，按既有 Gemini 出站口径渲染为文本
				if fallback := geminiPartFallbackText(part); fallback != "" {
					textBuf.WriteString(fallback)
					openai.EmitResponsesSSE(writer, "response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"content_index": contentIndex,
						"delta":         fallback,
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return billingUsage(), fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游断流未给收尾：仍合成 completed，避免客户端挂起
	finish()
	return billingUsage(), nil
}

// buildResponsesOutputFromGemini 构建 Responses 非流式响应的 output 数组。
// 文本合并为单个 message 项（居首），functionCall 各成一个 function_call 项。
func buildResponsesOutputFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) []map[string]any {
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
				id := geminiResponsesCallID(part.FunctionCall, info, toolIdx)
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
			"id":     fmt.Sprintf("msg_%s", info.RequestID),
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

// responsesIDFromGemini 生成 Responses 响应 ID，优先用上游 responseId
func responsesIDFromGemini(geminiResp *dto.GeminiChatResponse, info *common.RelayInfo) string {
	if geminiResp.ResponseID != "" {
		return fmt.Sprintf("resp_%s", geminiResp.ResponseID)
	}
	return fmt.Sprintf("resp_%s", info.RequestID)
}

// geminiResponsesCallID 取工具调用 ID：Gemini 的 functionCall.id 可选，缺失时合成。
// Responses 的 function_call 必须带 call_id，客户端下一轮按此 ID 回传结果。
func geminiResponsesCallID(fc *dto.GeminiFunctionCall, info *common.RelayInfo, idx int) string {
	if fc != nil && fc.ID != "" {
		return fc.ID
	}
	return fmt.Sprintf("call_%s_%d", info.RequestID, idx)
}

// marshalGeminiArgs 将 functionCall 参数序列化为 Responses 期望的 JSON 字符串
func marshalGeminiArgs(fc *dto.GeminiFunctionCall) string {
	b, err := json.Marshal(geminiFunctionArgs(fc))
	if err != nil {
		return "{}"
	}
	return string(b)
}
