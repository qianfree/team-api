package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/qianfree/team-api/relay/common"
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

	// 非 200：转换为 Responses 格式的错误
	if resp.StatusCode != http.StatusOK {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, nil
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

// chatCompletionToResponsesResponse 将 Chat Completions 响应转换为 Responses API 响应
func chatCompletionToResponsesResponse(chatResp *dto.ChatCompletionResponse, info *common.RelayInfo) *dto.OpenAIResponsesResponse {
	modelName := info.OriginModelName

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
		Instructions:       nil,
		MaxOutputTokens:    nil,
		Model:              modelName,
		Output:             output,
		ParallelToolCalls:  true,
		PreviousResponseID: nil,
		Reasoning:          &dto.ResponsesReasoning{Effort: nil, Summary: nil},
		Store:              true,
		Temperature:        float64Ptr(1.0),
		Text:               &dto.ResponsesText{Format: dto.ResponsesTextFormat{Type: "text"}},
		ToolChoice:         "auto",
		Tools:              make([]any, 0),
		TopP:               float64Ptr(1.0),
		Truncation:         "disabled",
		User:               nil,
		Metadata:           make(map[string]any),
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		_, _ = writer.Write(body)
		return &common.Usage{}, nil
	}

	helper.SetEventStreamHeaders(writer)
	var writeMu sync.Mutex
	defer helper.PingTicker(writer, 15*time.Second, &writeMu)()

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

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

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
				"response": buildResponsesObjectMap(respID, createdAt, "in_progress", modelName, []any{}, nil, nil),
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

			// tool calls
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

				// tool call arguments delta
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

	// 估算 usage
	if usage.CompletionTokens == 0 {
		text := contentBuilder.String()
		if len(text) > 0 {
			usage.CompletionTokens = len(text) / 4
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

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
		"response": buildResponsesObjectMap(respID, createdAt, "completed", modelName, finalOutput, buildResponsesUsageMap(&usage), &completedAt),
	})

	if info.StreamStatus.GetEndReason() == "" {
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	usage.CacheIncludedInPrompt = true
	return &usage, nil
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

// buildResponsesObjectMap 构建 Responses API response 对象的完整字段 map
func buildResponsesObjectMap(respID string, createdAt int, status string, model string, output any, usageObj map[string]any, completedAt *int) map[string]any {
	m := map[string]any{
		"id":                   respID,
		"object":               "response",
		"created_at":           createdAt,
		"status":               status,
		"error":                nil,
		"incomplete_details":   nil,
		"instructions":         nil,
		"max_output_tokens":    nil,
		"model":                model,
		"output":               output,
		"parallel_tool_calls":  true,
		"previous_response_id": nil,
		"reasoning":            map[string]any{"effort": nil, "summary": nil},
		"store":                true,
		"temperature":          1.0,
		"text":                 map[string]any{"format": map[string]any{"type": "text"}},
		"tool_choice":          "auto",
		"tools":                []any{},
		"top_p":                1.0,
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
