package native_responses

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// claudeToolCallState Responses 桥接中聚合中的工具调用（Claude tool_use 块）
type claudeToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// ClaudeToResponsesStreamConverter 将 Claude SSE 流转换为 OpenAI Responses 事件流。
// 事件序列与 chat→responses 桥接保持一致，每个事件以 StreamEvent{Event: 事件名} 输出。
type ClaudeToResponsesStreamConverter struct{}

func (c *ClaudeToResponsesStreamConverter) ID() string {
	return relayconvert.ResponseConverterClaudeMessagesToOAIResponsesStream
}

func (c *ClaudeToResponsesStreamConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToResponsesStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *ClaudeToResponsesStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Claude SSE 事件流转换为 Responses 事件帧序列。
func (c *ClaudeToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	createdAt := int(time.Now().Unix())
	modelName := originModelName(info)

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

	// emitEvent 发出一个 Responses 格式事件帧
	emitEvent := func(eventType string, data map[string]any, u *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: eventType, Data: data, Usage: u})
	}

	// closeTextPart 关闭文本 content part（进入工具调用或流结束时调用）
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		finishedText := textBuf.String()
		if err := emitEvent("response.output_text.done", map[string]any{
			"type":          "response.output_text.done",
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"text":          finishedText,
		}, nil); err != nil {
			return err
		}
		if err := emitEvent("response.content_part.done", map[string]any{
			"type":          "response.content_part.done",
			"item_id":       msgID,
			"output_index":  outputIndex,
			"content_index": contentIndex,
			"part": map[string]any{
				"type":        "output_text",
				"text":        finishedText,
				"annotations": []any{},
			},
		}, nil); err != nil {
			return err
		}
		if err := emitEvent("response.output_item.done", map[string]any{
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
		}, nil); err != nil {
			return err
		}
		sentTextDone = true
		outputIndex++
		return nil
	}

	// finish 发送每个工具调用的收尾事件 + response.completed（随帧携带计费用量）
	finish := func() error {
		if sentCompleted {
			return nil
		}
		if err := closeTextPart(); err != nil {
			return err
		}

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
			if err := emitEvent("response.function_call_arguments.done", map[string]any{
				"type":         "response.function_call_arguments.done",
				"item_id":      tc.id,
				"output_index": toolIndexByID[tc.id],
				"arguments":    tc.args.String(),
			}, nil); err != nil {
				return err
			}
			if err := emitEvent("response.output_item.done", map[string]any{
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
			}, nil); err != nil {
				return err
			}
			finalOutput = append(finalOutput, map[string]any{
				"type":      "function_call",
				"id":        tc.id,
				"call_id":   tc.id,
				"name":      tc.name,
				"arguments": tc.args.String(),
				"status":    "completed",
			})
		}

		// 客户端可见 usage 用 OpenAI 语义（input 含缓存，cached 为子集）；
		// completed 帧同时携带 Claude 计费口径的用量（input 不含缓存、cache 明细独立），
		// 输出缺失时的估算兜底（EstimateStreamOutputTokens）由宿主桥接层处理
		completedAt := int(time.Now().Unix())
		if err := emitEvent("response.completed", map[string]any{
			"type": "response.completed",
			"response": buildResponsesObjectMap(respID, createdAt, "completed", modelName,
				finalOutput, buildResponsesUsageMap(claudeVisibleUsage(&usage)), &completedAt, info),
		}, claudeBillingUsage(&usage)); err != nil {
			return err
		}
		sentCompleted = true
		return nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			// 流中断：计费兜底（输出估算、输入补齐）由宿主桥接层处理
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
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
				if event.Message.Model != "" && !isModelMapped(info) {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}
			if sentCreated {
				continue
			}
			if err := emitEvent("response.created", map[string]any{
				"type":     "response.created",
				"response": buildResponsesObjectMap(respID, createdAt, "in_progress", modelName, []any{}, nil, nil, info),
			}, nil); err != nil {
				return err
			}
			if err := emitEvent("response.output_item.added", map[string]any{
				"type":         "response.output_item.added",
				"output_index": outputIndex,
				"item": map[string]any{
					"type":    "message",
					"id":      msgID,
					"status":  "in_progress",
					"role":    "assistant",
					"content": []any{},
				},
			}, nil); err != nil {
				return err
			}
			if err := emitEvent("response.content_part.added", map[string]any{
				"type":          "response.content_part.added",
				"item_id":       msgID,
				"output_index":  outputIndex,
				"content_index": contentIndex,
				"part": map[string]any{
					"type":        "output_text",
					"text":        "",
					"annotations": []any{},
				},
			}, nil); err != nil {
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
				if err := emitEvent("response.output_item.added", map[string]any{
					"type":         "response.output_item.added",
					"output_index": outputIndex,
					"item": map[string]any{
						"type":    "function_call",
						"id":      tc.id,
						"call_id": tc.id,
						"name":    tc.name,
						"status":  "in_progress",
					},
				}, nil); err != nil {
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
					if err := emitEvent("response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"content_index": contentIndex,
						"delta":         *event.Delta.Text,
					}, nil); err != nil {
						return err
					}
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					if err := emitEvent("response.reasoning_summary_text.delta", map[string]any{
						"type":          "response.reasoning_summary_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"summary_index": 0,
						"delta":         *event.Delta.Thinking,
					}, nil); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" && currentTool != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
					if err := emitEvent("response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      currentTool.id,
						"output_index": toolIndexByID[currentTool.id],
						"delta":        *event.Delta.PartialJSON,
					}, nil); err != nil {
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
			return finish()

		case "error":
			// 上游 error 事件：不合成假成功的 response.completed，直接以错误返回；
			// 错误体写出与已开场/未开场的分支处理由宿主桥接层完成
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

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游未发 message_stop 即断流：仍合成 completed，避免客户端挂起
	return finish()
}
