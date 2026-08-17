package claude

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
)

// ========== Responses 入站桥接：Claude Messages → OpenAI Responses ==========
//
// 请求侧由 ConvertResponsesToClaude 完成（Responses → OpenAI → Claude），
// 这里做响应侧：把 Claude 上游的响应/SSE 转回 Responses 格式，
// 事件发射器复用 openai 包（EmitResponsesSSE / BuildResponsesObjectMap），
// 与 chat→responses 桥接（openai/responses.go）的事件序列保持一致。

// claudeToolCallState Responses 桥接中聚合中的工具调用（Claude tool_use 块）
type claudeToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// handleNonStreamToResponses 将 Claude 非流式响应转换为 Responses 格式
func (a *Adaptor) handleNonStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// relaykit 转换器路径优先；失败/未覆盖回退下方旧内联转换逻辑
	if responsesBody, usage, ok := relaykit_bridge.TryConvertResponsesResponseViaRelaykit(ctx, info, body); ok {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(responsesBody)
		return usage, nil
	}

	var claudeResp dto.ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "invalid response body", err).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}

	// 构建 output：文本块 → message 项，tool_use 块 → function_call 项
	var textParts []string
	output := make([]map[string]any, 0)
	for _, block := range claudeResp.Content {
		switch block.Type {
		case "text":
			if block.Text != nil && *block.Text != "" {
				textParts = append(textParts, *block.Text)
			}
		case "thinking", "redacted_thinking":
			// 思考内容无 Responses 非流式对应物，跳过（流式侧以 reasoning summary 事件透出）
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			output = append(output, map[string]any{
				"type":      "function_call",
				"id":        block.ID,
				"call_id":   block.ID,
				"name":      block.Name,
				"arguments": string(argsJSON),
				"status":    "completed",
			})
		}
	}
	if len(textParts) > 0 {
		msgItem := map[string]any{
			"type":   "message",
			"id":     fmt.Sprintf("msg_%s", claudeResp.ID),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{{
				"type":        "output_text",
				"text":        strings.Join(textParts, "\n"),
				"annotations": []any{},
			}},
		}
		output = append([]map[string]any{msgItem}, output...)
	}

	modelName := claudeResp.Model
	if modelName == "" || info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}
	respID := fmt.Sprintf("resp_%s", claudeResp.ID)
	if claudeResp.ID == "" {
		respID = fmt.Sprintf("resp_%d", time.Now().UnixNano())
	}
	createdAt := int(time.Now().Unix())
	completedAt := createdAt

	// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为子集）
	var visibleUsage *common.Usage
	if claudeResp.Usage != nil {
		promptTotal := claudeResp.Usage.InputTokens +
			claudeResp.Usage.CacheReadInputTokens +
			claudeResp.Usage.CacheCreationInputTokens
		visibleUsage = &common.Usage{
			PromptTokens:        promptTotal,
			CompletionTokens:    claudeResp.Usage.OutputTokens,
			TotalTokens:         promptTotal + claudeResp.Usage.OutputTokens,
			PromptTokensDetails: claudeUsageToTokenDetails(claudeResp.Usage),
		}
	} else {
		visibleUsage = &common.Usage{}
	}

	responsesBody, err := json.Marshal(openai.BuildResponsesObjectMap(respID, createdAt, "completed", modelName, output, openai.BuildResponsesUsageMap(visibleUsage), &completedAt, info))
	if err != nil {
		return nil, fmt.Errorf("marshal responses body failed: %w", err)
	}
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(responsesBody)

	// 计费返回值按 Claude 口径（input 不含缓存，cache 独立列）
	return buildUsageFromClaude(claudeResp.Usage), nil
}

// handleStreamToResponses 将 Claude 流式响应转换为 Responses 格式的 SSE
func (a *Adaptor) handleStreamToResponses(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil).WithRetryAfter(constant.RetryAfterFromHeader(resp.Header))
	}
	defer resp.Body.Close()

	helper.SetEventStreamHeaders(writer)
	writer = helper.NewSafeWriter(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	createdAt := int(time.Now().Unix())
	modelName := info.OriginModelName

	var usage dto.ClaudeUsage
	var textBuf strings.Builder
	sentCreated := false
	sentTextDone := false
	sentCompleted := false
	outputIndex := 0
	contentIndex := 0
	toolCalls := make([]*claudeToolCallState, 0) // 有序聚合，completed 的 output 数组按此顺序
	toolIndexByID := make(map[string]int)        // callID → output_index
	var currentTool *claudeToolCallState         // 正在接收参数增量的工具调用

	// buildBillingUsage 计费口径（Claude：input 不含缓存），输出缺失按已转发文本估算
	buildBillingUsage := func() *common.Usage {
		u := buildUsageFromClaude(&usage)
		if u.CompletionTokens == 0 && textBuf.Len() > 0 {
			u.CompletionTokens = helper.EstimateStreamOutputTokens(info, textBuf.Len())
			u.TotalTokens = u.PromptTokens + u.CompletionTokens
		}
		return u
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

	// finish 发送每个工具调用的收尾事件 + response.completed
	finish := func() {
		if sentCompleted {
			return
		}
		closeTextPart()

		// 输出数组：文本消息（closeTextPart 已置 sentTextDone，此处等价于有文本才保留，
		// 与 chat→responses 桥接的 finalOutput 构建口径一致）
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
				"arguments":    tc.args.String(),
			})
			openai.EmitResponsesSSE(writer, "response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": toolIndexByID[tc.id],
				"item": map[string]any{
					"type":      "function_call",
					"id":        tc.id,
					"call_id":   tc.id,
					"name":      tc.name,
					"arguments": tc.args.String(),
					"status":    "completed",
				},
			})
			finalOutput = append(finalOutput, map[string]any{
				"type":      "function_call",
				"id":        tc.id,
				"call_id":   tc.id,
				"name":      tc.name,
				"arguments": tc.args.String(),
				"status":    "completed",
			})
		}

		// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为子集）
		promptTotal := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
		visibleUsage := &common.Usage{
			PromptTokens:        promptTotal,
			CompletionTokens:    usage.OutputTokens,
			TotalTokens:         promptTotal + usage.OutputTokens,
			PromptTokensDetails: claudeUsageToTokenDetails(&usage),
		}
		completedAt := int(time.Now().Unix())
		openai.EmitResponsesSSE(writer, "response.completed", map[string]any{
			"type":     "response.completed",
			"response": openai.BuildResponsesObjectMap(respID, createdAt, "completed", modelName, finalOutput, openai.BuildResponsesUsageMap(visibleUsage), &completedAt, info),
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
			interruptedUsage := buildUsageFromClaude(&usage)
			helper.ApplyInterruptedUsageFallback(info, interruptedUsage, textBuf.Len())
			return interruptedUsage, common.ErrStreamInterrupted
		default:
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data, _ := helper.ExtractSSEData(line)
		if data == "" || data == "[DONE]" {
			continue
		}
		info.SetFirstResponseTime()

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				if event.Message.Model != "" && !info.ChannelMeta.IsModelMapped {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}
			if sentCreated {
				continue
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

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			switch event.ContentBlock.Type {
			case "tool_use":
				// 先关闭文本 content part，再开 function_call 项
				closeTextPart()
				tc := &claudeToolCallState{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
				toolCalls = append(toolCalls, tc)
				toolIndexByID[tc.id] = outputIndex
				currentTool = tc
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
				outputIndex++
			case "text", "thinking", "redacted_thinking":
				// 文本/思考块：文本复用首个 content part，思考以 reasoning summary 事件透出
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					textBuf.WriteString(*event.Delta.Text)
					openai.EmitResponsesSSE(writer, "response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"content_index": contentIndex,
						"delta":         *event.Delta.Text,
					})
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					openai.EmitResponsesSSE(writer, "response.reasoning_summary_text.delta", map[string]any{
						"type":          "response.reasoning_summary_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"summary_index": 0,
						"delta":         *event.Delta.Thinking,
					})
				}
			case "input_json_delta":
				if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" && currentTool != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
					openai.EmitResponsesSSE(writer, "response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      currentTool.id,
						"output_index": toolIndexByID[currentTool.id],
						"delta":        *event.Delta.PartialJSON,
					})
				}
			case "signature_delta":
				// 思考签名无 Responses 对应物，忽略
			}

		case "content_block_stop":
			// 块级收尾统一延迟到 message_stop / finish，这里仅结束当前工具块的增量定向
			if currentTool != nil {
				currentTool = nil
			}

		case "message_delta":
			if event.Usage != nil {
				if event.Usage.InputTokens > 0 {
					usage.InputTokens = event.Usage.InputTokens
				}
				usage.OutputTokens = event.Usage.OutputTokens
				if event.Usage.CacheReadInputTokens > 0 {
					usage.CacheReadInputTokens = event.Usage.CacheReadInputTokens
				}
				if event.Usage.CacheCreationInputTokens > 0 {
					usage.CacheCreationInputTokens = event.Usage.CacheCreationInputTokens
				}
				if event.Usage.CacheCreation != nil {
					usage.CacheCreation = event.Usage.CacheCreation
				}
			}

		case "message_stop":
			finish()
			return buildBillingUsage(), nil

		case "error":
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = fmt.Sprintf("claude stream error: %s", string(b))
				}
			}
			info.StreamStatus.SetEndReason(common.StreamEndReasonError, fmt.Errorf("%s", errMsg))
			g.Log().Warningf(ctx, "[Claude.handleStreamToResponses] %v", errMsg)
			if !sentCreated {
				errBody, _ := json.Marshal(map[string]any{
					"error": map[string]any{
						"message": errMsg,
						"type":    "upstream_error",
						"param":   nil,
						"code":    "upstream_stream_error",
					},
				})
				writeResponsesErrorBody(writer, errBody)
				upstreamErr := constant.NewUpstreamError(http.StatusBadGateway, string(errBody), nil)
				upstreamErr.ResponseWritten = true
				return buildBillingUsage(), upstreamErr
			}
			return buildBillingUsage(), constant.NewUpstreamError(http.StatusBadGateway, errMsg, nil)

		case "ping":
			// 保活事件，忽略
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return buildBillingUsage(), fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游未发 message_stop 即断流：仍合成 completed，避免客户端挂起
	finish()

	// 计费返回值按 Claude 口径（input 不含缓存，cache 独立列）
	return buildBillingUsage(), nil
}

// writeResponsesErrorBody 写入 Responses 兼容的错误响应体（SSE 头提交后状态码已固定 200）
func writeResponsesErrorBody(writer http.ResponseWriter, body []byte) {
	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(body)
	if f, ok := writer.(http.Flusher); ok {
		f.Flush()
	}
}
