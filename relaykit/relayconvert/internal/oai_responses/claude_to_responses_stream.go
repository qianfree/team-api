package oai_responses

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
)

// ClaudeToResponsesStreamConverter Claude 上游 SSE → Responses 客户端 SSE（流式响应侧）。
// 移植自宿主 relay/channel/claude/responses_bridge.go 的 handleStreamToResponses 状态机：
// 文本/思考增量、工具调用聚合（claudeToolCallState）、事件收尾顺序与
// chat→responses 桥接保持一致。宿主职责（SSE 头/保活/错误体写出/计费）不在本转换器内。
type ClaudeToResponsesStreamConverter struct{}

// ConvertStreamResponse 读取 Claude SSE reader，经 chunkWriter 输出 *dto.ResponsesStreamEvent。
// 宿主据此写出 `event: <type>\ndata: <json>` 格式的事件流。
func (c *ClaudeToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	respID := fmt.Sprintf("resp_%d", NowFunc().UnixNano())
	msgID := fmt.Sprintf("msg_%d", NowFunc().UnixNano())
	createdAt := int(NowFunc().Unix())
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

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
	echo := responsesEchoOf(info)

	// emit 输出一个 Responses SSE 事件（chunkWriter 出错即客户端写失败，终止转换）
	emit := func(eventType string, payload map[string]any) error {
		payload["type"] = eventType
		return chunkWriter(&dto.ResponsesStreamEvent{Type: eventType, Data: payload})
	}

	// closeTextPart 关闭文本 content part（进入工具调用或流结束时调用）
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		finishedText := textBuf.String()
		if err := emit("response.output_text.done", map[string]any{
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"text":          finishedText,
		}); err != nil {
			return err
		}
		if err := emit("response.content_part.done", map[string]any{
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        finishedText,
				"annotations": []any{},
			},
		}); err != nil {
			return err
		}
		if err := emit("response.output_item.done", map[string]any{
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
		}); err != nil {
			return err
		}
		sentTextDone = true
		outputIndex++
		return nil
	}

	// finish 发送每个工具调用的收尾事件 + response.completed
	finish := func() error {
		if sentCompleted {
			return nil
		}
		if err := closeTextPart(); err != nil {
			return err
		}

		// 输出数组：文本消息（有文本才保留），与 chat→responses 桥接的 finalOutput 构建口径一致
		finalOutput := make([]dto.ResponsesOutput, 0)
		if textBuf.Len() > 0 {
			finalOutput = append(finalOutput, dto.ResponsesOutput{
				Type:   "message",
				ID:     msgID,
				Status: "completed",
				Role:   "assistant",
				Content: []dto.ResponsesOutputContent{{
					Type:        "output_text",
					Text:        textBuf.String(),
					Annotations: []dto.ResponsesAnnotation{},
				}},
			})
		}

		for _, tc := range toolCalls {
			if err := emit("response.function_call_arguments.done", map[string]any{
				"item_id":      tc.id,
				"output_index": toolIndexByID[tc.id],
				"arguments":    tc.args.String(),
			}); err != nil {
				return err
			}
			if err := emit("response.output_item.done", map[string]any{
				"output_index": toolIndexByID[tc.id],
				"item": map[string]any{
					"type":      "function_call",
					"id":        tc.id,
					"call_id":   tc.id,
					"name":      tc.name,
					"arguments": tc.args.String(),
					"status":    "completed",
				},
			}); err != nil {
				return err
			}
			finalOutput = append(finalOutput, dto.ResponsesOutput{
				Type:      "function_call",
				ID:        tc.id,
				CallID:    tc.id,
				Name:      tc.name,
				Arguments: tc.args.String(),
				Status:    "completed",
			})
		}

		// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为其子集）
		promptTotal := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
		visibleUsage := &dto.UsageWithDetails{
			PromptTokens:     promptTotal,
			CompletionTokens: usage.OutputTokens,
			TotalTokens:      promptTotal + usage.OutputTokens,
		}
		if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
			visibleUsage.PromptTokensDetails = claudeUsageToTokenDetails(&usage)
		}
		completedAt := int(NowFunc().Unix())
		return emit("response.completed", map[string]any{
			"response": buildResponsesObject(respID, createdAt, "completed", modelName, finalOutput,
				responsesUsageOf(visibleUsage), &completedAt, echo),
		})
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractSSEData(line)
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				//（旧路径仅在未做模型映射时采用上游模型名，convmeta 无该信号，
				// 列为已知差异：映射渠道时采用上游返回的模型名）
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}
			if sentCreated {
				continue
			}
			if err := emit("response.created", map[string]any{
				"response": buildResponsesObject(respID, createdAt, "in_progress", modelName, []dto.ResponsesOutput{}, nil, nil, echo),
			}); err != nil {
				return err
			}
			if err := emit("response.output_item.added", map[string]any{
				"output_index": outputIndex,
				"item": map[string]any{
					"type":    "message",
					"id":      msgID,
					"status":  "in_progress",
					"role":    "assistant",
					"content": []any{},
				},
			}); err != nil {
				return err
			}
			if err := emit("response.content_part.added", map[string]any{
				"item_id":       msgID,
				"output_index":  outputIndex,
				"content_index": contentIndex,
				"part": map[string]any{
					"type":        "output_text",
					"text":        "",
					"annotations": []any{},
				},
			}); err != nil {
				return err
			}
			sentCreated = true

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			switch event.ContentBlock.Type {
			case "tool_use":
				// 先关闭文本 content part，再开 function_call 项
				if err := closeTextPart(); err != nil {
					return err
				}
				tc := &claudeToolCallState{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
				toolCalls = append(toolCalls, tc)
				toolIndexByID[tc.id] = outputIndex
				currentTool = tc
				if err := emit("response.output_item.added", map[string]any{
					"output_index": outputIndex,
					"item": map[string]any{
						"type":    "function_call",
						"id":      tc.id,
						"call_id": tc.id,
						"name":    tc.name,
						"status":  "in_progress",
					},
				}); err != nil {
					return err
				}
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
					if err := emit("response.output_text.delta", map[string]any{
						"item_id":       msgID,
						"output_index":  0,
						"content_index": contentIndex,
						"delta":         *event.Delta.Text,
					}); err != nil {
						return err
					}
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					if err := emit("response.reasoning_summary_text.delta", map[string]any{
						"item_id":       msgID,
						"output_index":  0,
						"summary_index": 0,
						"delta":         *event.Delta.Thinking,
					}); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" && currentTool != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
					if err := emit("response.function_call_arguments.delta", map[string]any{
						"item_id":      currentTool.id,
						"output_index": toolIndexByID[currentTool.id],
						"delta":        *event.Delta.PartialJSON,
					}); err != nil {
						return err
					}
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
			if err := finish(); err != nil {
				return err
			}
			sentCompleted = true
			return nil

		case "error":
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = fmt.Sprintf("claude stream error: %s", string(b))
				}
			}
			return fmt.Errorf("%s", errMsg)

		case "ping":
			// 保活事件，忽略
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游未发 message_stop 即断流：仍合成 completed，避免客户端挂起
	if err := finish(); err != nil {
		return err
	}
	return nil
}

// claudeToolCallState 流式中聚合的工具调用（Claude tool_use 块）
type claudeToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// extractSSEData 从 SSE data 行提取数据部分（与宿主 helper.ExtractSSEData 等价的纯实现）。
func extractSSEData(line string) string {
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data)
}
