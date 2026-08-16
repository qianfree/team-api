package openai

import (
	"bufio"
	"bytes"
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
)

// ========== 响应转换：Chat Completions → Responses ==========

// handleResponsesInboundNonStream 将 Chat Completions 非流式响应转换为 Responses 格式
func (a *Adaptor) handleResponsesInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	// 非 200：透传上游错误响应并返回上游错误（驱动重试/渠道健康上报）
	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	// 解析 Chat Completions 响应
	var chatResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, nil
	}

	// 转换为 Responses 格式
	responsesResp := chatCompletionToResponsesResponse(&chatResp, info)
	responsesBody, err := json.Marshal(responsesResp)
	if err != nil {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, nil
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responsesBody)

	return &common.Usage{
		PromptTokens:          chatResp.Usage.PromptTokens,
		CompletionTokens:      chatResp.Usage.CompletionTokens,
		TotalTokens:           chatResp.Usage.TotalTokens,
		CacheIncludedInPrompt: true,
	}, nil
}

// responsesRequestEcho 合成 Responses 响应时需回显（echo）的请求参数
type responsesRequestEcho struct {
	temperature     *float64
	topP            *float64
	maxOutputTokens *int
	instructions    any
}

// extractResponsesRequestEcho 从 info.ResponsesRequest 提取合成响应应 echo 的请求参数。
// 快照缺失（直连路径/异常）时回退 OpenAI 默认值（temperature=1.0 / top_p=1.0，其余 nil）。
func extractResponsesRequestEcho(info *common.RelayInfo) responsesRequestEcho {
	echo := responsesRequestEcho{temperature: float64Ptr(1.0), topP: float64Ptr(1.0)}
	if info == nil || info.ResponsesRequest == nil {
		return echo
	}
	rr := info.ResponsesRequest
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
func chatCompletionToResponsesResponse(chatResp *dto.ChatCompletionResponse, info *common.RelayInfo) *dto.OpenAIResponsesResponse {
	modelName := info.OriginModelName
	echo := extractResponsesRequestEcho(info)

	// 构建 output
	output := make([]dto.ResponsesOutput, 0)
	for _, choice := range chatResp.Choices {
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
			InputTokens:  chatResp.Usage.PromptTokens,
			OutputTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:  chatResp.Usage.TotalTokens,
			InputTokensDetails: &dto.InputTokenDetails{
				CachedTokens: chatResp.Usage.PromptTokensDetails.CachedTokens,
				AudioTokens:  chatResp.Usage.PromptTokensDetails.AudioTokens,
			},
			OutputTokenDetails: &dto.OutputTokenDetails{
				ReasoningTokens:          chatResp.Usage.CompletionTokenDetails.ReasoningTokens,
				AcceptedPredictionTokens: chatResp.Usage.CompletionTokenDetails.AcceptedPredictionTokens,
				RejectedPredictionTokens: chatResp.Usage.CompletionTokenDetails.RejectedPredictionTokens,
			},
		},
	}
}

// ========== 流式响应转换：Chat Completions SSE → Responses SSE ==========

// handleResponsesInboundStream 将 Chat Completions 流式响应转换为 Responses 格式的 SSE
func (a *Adaptor) handleResponsesInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体），透传并返回上游错误驱动重试
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := info.OriginModelName
	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	createdAt := int(time.Now().Unix())

	var usage common.Usage
	var contentBuilder strings.Builder
	sentCreated := false
	sentTextDone := false
	parsedChunks := 0
	outputIndex := 0
	contentIndex := 0
	toolCallIndexByID := make(map[string]int)
	toolCallArgsByID := make(map[string]string)
	toolCallNameByID := make(map[string]string)
	// 通过 index 追踪 tool call ID（OpenAI 流式中后续 chunk 的 ID 为空，只有 index）
	toolCallIDByIndex := make(map[int]string)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断计费兜底：输出缺失按已转发文本 2 字符/token 估算，输入用请求侧估算值补齐
			helper.ApplyInterruptedUsageFallback(info, &usage, contentBuilder.Len())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data, _ := helper.ExtractSSEData(line)

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
		}
		if data == "[DONE]" {
			break
		}

		// 检测 SSE 流中内嵌的上游错误对象：部分聚合商出错时返回 HTTP 200 + SSE，
		// 错误信息夹在 data 行里（{"error":{...}}）。不识别会被当作解析失败静默丢弃，
		// 最终合成空的 response.completed（客户端表现为"成功但无内容"）。
		if errBody, ok := extractStreamEmbeddedError([]byte(data)); ok {
			err := fmt.Errorf("upstream embedded error in SSE stream: %.500s", string(errBody))
			g.Log().Warningf(ctx, "[OpenAI.handleResponsesInboundStream] %v", err)
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			if !sentCreated {
				// 尚未向客户端发送任何事件：透传上游错误体（与 SSE 开始前的非 200 分支行为一致）
				writeUpstreamErrorResponse(writer, resp.StatusCode, []byte(data))
				upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(errBody), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
				upstreamErr.ResponseWritten = true
				return &usage, upstreamErr
			}
			// 已发送部分事件：流已污染无法回写错误体，直接返回错误终止处理
			return &usage, constant.NewUpstreamError(resp.StatusCode, string(errBody), nil)
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 解析失败不静默丢弃：记录日志便于定位上游返回非 chat 格式的问题
			// （如 responses 格式 SSE、纯 JSON 非流式体、空 body 等）
			g.Log().Warningf(ctx, "[OpenAI.handleResponsesInboundStream] unmarshal chat stream chunk failed: %v, data: %.200s", err, data)
			continue
		}
		parsedChunks++

		// 第一个 chunk：发送 response.created + output_item.added + content_part.added
		if !sentCreated {
			if chunk.ID != "" {
				respID = fmt.Sprintf("resp_%s", chunk.ID)
				msgID = fmt.Sprintf("msg_%s", chunk.ID)
			}
			if chunk.Created > 0 {
				createdAt = int(chunk.Created)
			}
			if chunk.Model != "" && !info.ChannelMeta.IsModelMapped {
				modelName = chunk.Model
			}

			// response.created
			emitResponsesSSE(writer, "response.created", map[string]any{
				"type":     "response.created",
				"response": buildResponsesObjectMap(respID, createdAt, "in_progress", modelName, []any{}, nil, nil, info),
			})

			// response.output_item.added
			emitResponsesSSE(writer, "response.output_item.added", map[string]any{
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

			// response.content_part.added
			emitResponsesSSE(writer, "response.content_part.added", map[string]any{
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

		// 提取 usage
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		// 处理 choices delta
		for _, choice := range chunk.Choices {
			// 文本内容 delta
			if choice.Delta.Content != nil {
				var deltaText string
				switch v := choice.Delta.Content.(type) {
				case string:
					deltaText = v
				}
				if deltaText != "" {
					contentBuilder.WriteString(deltaText)
					emitResponsesSSE(writer, "response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  outputIndex,
						"content_index": contentIndex,
						"delta":         deltaText,
					})
				}
			}

			// 推理内容
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				emitResponsesSSE(writer, "response.reasoning_summary_text.delta", map[string]any{
					"type":          "response.reasoning_summary_text.delta",
					"item_id":       msgID,
					"output_index":  outputIndex,
					"summary_index": 0,
					"delta":         *choice.Delta.ReasoningContent,
				})
			}

			// 工具调用
			for _, tc := range choice.Delta.ToolCalls {
				callID := tc.ID

				// 新 tool call：有 ID 和 name
				if callID != "" && tc.Function.Name != "" {
					// 记录 index → callID 映射，用于后续参数 chunk 的查找
					toolCallIDByIndex[tc.Index] = callID

					// 先关闭文本 content part
					if !sentTextDone {
						finishedText := contentBuilder.String()
						emitResponsesSSE(writer, "response.output_text.done", map[string]any{
							"type":          "response.output_text.done",
							"item_id":       msgID,
							"output_index":  outputIndex,
							"content_index": contentIndex,
							"text":          finishedText,
						})
						emitResponsesSSE(writer, "response.content_part.done", map[string]any{
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
						emitResponsesSSE(writer, "response.output_item.done", map[string]any{
							"type":         "response.output_item.done",
							"output_index": outputIndex,
							"item": map[string]any{
								"type":   "message",
								"id":     msgID,
								"status": "completed",
								"role":   "assistant",
								"content": []map[string]any{
									{
										"type":        "output_text",
										"text":        finishedText,
										"annotations": []any{},
									},
								},
							},
						})
						sentTextDone = true
						outputIndex++
					}

					toolCallIndexByID[callID] = outputIndex
					toolCallNameByID[callID] = tc.Function.Name
					toolCallArgsByID[callID] = ""

					emitResponsesSSE(writer, "response.output_item.added", map[string]any{
						"type":         "response.output_item.added",
						"output_index": outputIndex,
						"item": map[string]any{
							"type":    "function_call",
							"id":      callID,
							"call_id": callID,
							"name":    tc.Function.Name,
							"status":  "in_progress",
						},
					})
					outputIndex++
				}

				// 参数 chunk：ID 可能为空，通过 index 查找对应的 callID
				if callID == "" {
					callID = toolCallIDByIndex[tc.Index]
				}
				if callID == "" {
					continue
				}

				// 工具调用 arguments 增量
				if tc.Function.Arguments != "" {
					toolCallArgsByID[callID] += tc.Function.Arguments
					emitResponsesSSE(writer, "response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      callID,
						"output_index": toolCallIndexByID[callID],
						"delta":        tc.Function.Arguments,
					})
				}
			}

			// finish_reason
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishedText := contentBuilder.String()

				// 关闭文本 content part（如果尚未关闭）
				if !sentTextDone {
					emitResponsesSSE(writer, "response.output_text.done", map[string]any{
						"type":          "response.output_text.done",
						"item_id":       msgID,
						"output_index":  outputIndex,
						"content_index": contentIndex,
						"text":          finishedText,
					})
					emitResponsesSSE(writer, "response.content_part.done", map[string]any{
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
					emitResponsesSSE(writer, "response.output_item.done", map[string]any{
						"type":         "response.output_item.done",
						"output_index": outputIndex,
						"item": map[string]any{
							"type":   "message",
							"id":     msgID,
							"status": "completed",
							"role":   "assistant",
							"content": []map[string]any{
								{
									"type":        "output_text",
									"text":        finishedText,
									"annotations": []any{},
								},
							},
						},
					})
				}

				// 发送每个 tool call 的 function_call_arguments.done + output_item.done
				for tcID, tcIdx := range toolCallIndexByID {
					emitResponsesSSE(writer, "response.function_call_arguments.done", map[string]any{
						"type":         "response.function_call_arguments.done",
						"item_id":      tcID,
						"output_index": tcIdx,
						"arguments":    toolCallArgsByID[tcID],
					})
					emitResponsesSSE(writer, "response.output_item.done", map[string]any{
						"type":         "response.output_item.done",
						"output_index": tcIdx,
						"item": map[string]any{
							"type":      "function_call",
							"id":        tcID,
							"call_id":   tcID,
							"name":      toolCallNameByID[tcID],
							"arguments": toolCallArgsByID[tcID],
							"status":    "completed",
						},
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF && ctx.Err() == nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			return &usage, fmt.Errorf("stream scanner error: %w", err)
		}
	}

	// 上游零有效 chunk 时不再静默合成空响应：记录告警便于定位
	// （上游返回非 SSE 体 / 非 chat 格式 / 空 body / 纯 [DONE] 等异常都会走到这里）
	if !sentCreated {
		g.Log().Warningf(ctx,
			"[OpenAI.handleResponsesInboundStream] upstream stream yielded no parseable chat chunks (parsed=%d), synthesizing empty response.completed: channel=%d(%s) model=%s request_id=%s",
			parsedChunks, info.ChannelMeta.ChannelID, info.ChannelMeta.ChannelName, info.OriginModelName, info.RequestID)
	}

	// 估算 usage（正常结束 4 字符/token；scanner 异常等部分传输场景 2 字符/token）
	if usage.CompletionTokens == 0 {
		text := contentBuilder.String()
		if len(text) > 0 {
			usage.CompletionTokens = helper.EstimateStreamOutputTokens(info, len(text))
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	// 流中断：输入 token 用请求侧估算值补齐，保证输入正常计费
	helper.ApplyInterruptedUsageFallback(info, &usage, contentBuilder.Len())

	// 构建 response.completed 的 output 数组（包含文本消息 + 所有 tool call）
	finalOutput := make([]map[string]any, 0)
	if !sentTextDone || contentBuilder.Len() > 0 {
		finalOutput = append(finalOutput, map[string]any{
			"type":   "message",
			"id":     msgID,
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{
					"type":        "output_text",
					"text":        contentBuilder.String(),
					"annotations": []any{},
				},
			},
		})
	}
	for tcID := range toolCallIndexByID {
		finalOutput = append(finalOutput, map[string]any{
			"type":      "function_call",
			"id":        tcID,
			"call_id":   tcID,
			"name":      toolCallNameByID[tcID],
			"arguments": toolCallArgsByID[tcID],
			"status":    "completed",
		})
	}

	// response.completed
	completedAt := int(time.Now().Unix())
	emitResponsesSSE(writer, "response.completed", map[string]any{
		"type":     "response.completed",
		"response": buildResponsesObjectMap(respID, createdAt, "completed", modelName, finalOutput, buildResponsesUsageMap(&usage), &completedAt, info),
	})

	if info.StreamStatus.GetEndReason() == "" {
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	usage.CacheIncludedInPrompt = true
	return &usage, nil
}

// extractStreamEmbeddedError 检测 SSE data 行中内嵌的上游错误对象（存在 "error" 键且值非 null）。
// 与 isUpstreamOpenAIError 的区别：后者用于完整响应体且不区分 null；这里对流式 chunk 逐行
// 检测，并排除 "error":null（部分供应商的正常 chunk 会携带空 error 字段）。
func extractStreamEmbeddedError(data []byte) (json.RawMessage, bool) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, false
	}
	errBody, ok := raw["error"]
	if !ok {
		return nil, false
	}
	trimmed := string(bytes.TrimSpace(errBody))
	if trimmed == "" || trimmed == "null" {
		return nil, false
	}
	return errBody, true
}

// emitResponsesSSE 发送一个 Responses API 格式的 SSE 事件
func emitResponsesSSE(w http.ResponseWriter, eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(jsonData))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

// buildResponsesObjectMap 构建 Responses API response 对象的完整字段 map。
// 请求参数从 info.ResponsesRequest echo（快照缺失时回退默认值）；
// store 恒为 false——合成响应不落上游存储，客户端不可经生命周期端点 retrieve。
func buildResponsesObjectMap(respID string, createdAt int, status string, model string, output any, usageObj map[string]any, completedAt *int, info *common.RelayInfo) map[string]any {
	echo := extractResponsesRequestEcho(info)
	m := map[string]any{
		"id":                   respID,
		"object":               "response",
		"created_at":           createdAt,
		"status":               status,
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         echo.instructions,
		"max_output_tokens":    echo.maxOutputTokens,
		"model":                model,
		"output":               output,
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"reasoning":            map[string]any{"effort": nil, "summary": nil},
		"store":                false,
		"temperature":          *echo.temperature,
		"text":                 map[string]any{"format": map[string]any{"type": "text"}},
		"tool_choice":          "auto",
		"tools":                []any{},
		"top_p":                *echo.topP,
		"truncation":           "disabled",
		"user":                 nil,
		"metadata":             map[string]any{},
	}
	if completedAt != nil {
		m["completed_at"] = *completedAt
	}
	if usageObj != nil {
		m["usage"] = usageObj
	}
	return m
}

// buildResponsesUsageMap 构建 Responses API usage 对象
func buildResponsesUsageMap(usage *common.Usage) map[string]any {
	inputDetails := map[string]any{"cached_tokens": 0}
	outputDetails := map[string]any{"reasoning_tokens": 0}
	if usage.PromptTokensDetails != nil {
		inputDetails = map[string]any{
			"cached_tokens": usage.PromptTokensDetails.CachedTokens,
			"audio_tokens":  usage.PromptTokensDetails.AudioTokens,
		}
	}
	if usage.CompletionTokenDetails != nil {
		outputDetails = map[string]any{
			"reasoning_tokens":           usage.CompletionTokenDetails.ReasoningTokens,
			"audio_tokens":               usage.CompletionTokenDetails.AudioTokens,
			"accepted_prediction_tokens": usage.CompletionTokenDetails.AcceptedPredictionTokens,
			"rejected_prediction_tokens": usage.CompletionTokenDetails.RejectedPredictionTokens,
		}
	}
	return map[string]any{
		"input_tokens":          usage.PromptTokens,
		"output_tokens":         usage.CompletionTokens,
		"total_tokens":          usage.TotalTokens,
		"input_tokens_details":  inputDetails,
		"output_tokens_details": outputDetails,
	}
}

// float64Ptr 返回 float64 的指针
func float64Ptr(v float64) *float64 {
	return &v
}

// ========== 上游为 Responses 协议：Responses 响应原样透传 ==========

// responsesUsageToCommon 将 Responses API usage 转换为 common.Usage。
// OpenAI 原生 API 的 input_tokens 已含缓存 token（cache 是其子集），故 CacheIncludedInPrompt=true。
func responsesUsageToCommon(u *dto.ResponsesUsage) *common.Usage {
	usage := &common.Usage{CacheIncludedInPrompt: true}
	if u == nil {
		return usage
	}
	usage.PromptTokens = u.InputTokens
	usage.CompletionTokens = u.OutputTokens
	usage.TotalTokens = u.TotalTokens
	if d := u.InputTokensDetails; d != nil {
		usage.PromptTokensDetails = &common.TokenDetails{
			CachedTokens: d.CachedTokens,
			TextTokens:   d.TextTokens,
			AudioTokens:  d.AudioTokens,
			ImageTokens:  d.ImageTokens,
		}
	}
	if d := u.OutputTokenDetails; d != nil {
		usage.CompletionTokenDetails = &common.TokenDetails{
			TextTokens:               d.TextTokens,
			AudioTokens:              d.AudioTokens,
			ReasoningTokens:          d.ReasoningTokens,
			AcceptedPredictionTokens: d.AcceptedPredictionTokens,
			RejectedPredictionTokens: d.RejectedPredictionTokens,
		}
	}
	return usage
}

// recordResponseRoute 记录 response_id → 渠道路由（Redis），供 GET/DELETE/cancel
// 生命周期端点还原原始请求落到的渠道。ModelName 存 lookupModel 口径（BaseModelName），
// 与 MaterializeSelection 入参一致。
func recordResponseRoute(ctx context.Context, info *common.RelayInfo, responseID string) {
	if responseID == "" || info == nil || info.ChannelMeta == nil {
		return
	}
	common.DefaultResponseRouteStore.Record(ctx, info.TenantID, responseID, common.ResponseRoute{
		ChannelID: info.ChannelMeta.ChannelID,
		ModelName: info.BaseModelName,
	})
}

// handleResponsesUpstreamNonStream 上游为 Responses 协议时的非流式响应：
// 解析 usage 后原样透传上游响应体（模型映射时回写客户端请求的模型名）。
func (a *Adaptor) handleResponsesUpstreamNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// 非 200：透传上游错误响应并返回上游错误（驱动重试/渠道健康上报，同 handleChatNonStreamResponse）
	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamErrorFromResponse(resp, body)
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	// 解析 usage（计费用），响应体仍透传上游原始内容
	var responsesResp dto.OpenAIResponsesResponse
	if err := json.Unmarshal(body, &responsesResp); err == nil {
		recordResponseRoute(ctx, info, responsesResp.ID)
		return responsesUsageToCommon(responsesResp.Usage), nil
	}
	return &common.Usage{}, nil
}

// handleResponsesUpstreamStream 上游为 Responses 协议时的流式响应：
// 逐行原样透传 SSE（含 event: 行），从 response.completed / response.done 事件解析 usage。
func (a *Adaptor) handleResponsesUpstreamStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	// 非 200：上游在 SSE 开始前返回错误（非 SSE 体），透传并返回上游错误驱动重试
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

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var usage common.Usage
	var contentBuilder strings.Builder

	flush := func() {
		if f, ok := writer.(http.Flusher); ok {
			f.Flush()
		}
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			// 流中断计费兜底：输出缺失按已转发文本 2 字符/token 估算，输入用请求侧估算值补齐
			helper.ApplyInterruptedUsageFallback(info, &usage, contentBuilder.Len())
			return &usage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" {
			fmt.Fprintf(writer, "\n")
			flush()
			continue
		}
		// 原样透传 event: 行
		if strings.HasPrefix(line, "event:") {
			fmt.Fprintf(writer, "%s\n", line)
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
		}

		// 解析 data 提取 usage（不影响原样透传）
		if data != "[DONE]" {
			var streamResp dto.ResponsesStreamResponse
			if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
				switch streamResp.Type {
				case "response.created", "response.completed", "response.done":
					if r := streamResp.Response; r != nil {
						// 路由记录：created 即记录使 cancel 尽早可用，completed/done 刷新 TTL（SET 幂等）
						recordResponseRoute(ctx, info, r.ID)
						if streamResp.Type != "response.created" && r.Usage != nil {
							if u := responsesUsageToCommon(r.Usage); u != nil {
								usage = *u
							}
						}
					}
				case "response.output_text.delta":
					contentBuilder.WriteString(streamResp.Delta)
				}
			}
		}

		// 模型映射时回写客户端请求的模型名：response.created / response.completed 等事件携带上游模型名，
		// 直连透传前替换（同 chat StreamHandler 的逐行替换）
		outLine := line
		if info.ChannelMeta.IsModelMapped && data != "" && data != "[DONE]" {
			outLine = "data: " + string(helper.ReplaceModelName([]byte(data), info.OriginModelName))
		}

		fmt.Fprintf(writer, "%s\n", outLine)
		flush()

		if data == "[DONE]" {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		if err != io.EOF && ctx.Err() == nil {
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
			return &usage, fmt.Errorf("stream scanner error: %w", err)
		}
	}

	// 估算 usage（正常结束 4 字符/token；异常部分传输 2 字符/token）
	if usage.CompletionTokens == 0 {
		text := contentBuilder.String()
		if len(text) > 0 {
			usage.CompletionTokens = helper.EstimateStreamOutputTokens(info, len(text))
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	usage.CacheIncludedInPrompt = true

	if info.StreamStatus.GetEndReason() == "" {
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	return &usage, nil
}
