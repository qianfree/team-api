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
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
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
	contentIndex := 0
	// 输出项索引：message 项固定占 0，其余输出项（reasoning / tool call / server tool）
	// 由 itemIdx 从 1 起分配——与 chat→responses 桥接的口径一致
	itemIdx := 1
	toolCalls := make([]*claudeToolCallState, 0) // 有序聚合，completed 的 output 数组按此顺序
	toolIndexByID := make(map[string]int)        // callID → output_index
	var currentTool *claudeToolCallState         // 正在接收参数增量的工具调用
	// 服务端工具（claude 的 web_search）：映射为 Responses 的 web_search_call 项。
	// 搜索结果块（web_search_tool_result）无 Responses 对应物——结果已内化为模型的
	// 文本回答与引用，跳过即可
	serverToolCalls := make([]*claudeToolCallState, 0)
	serverToolIndexByID := make(map[string]int)
	var currentServerTool *claudeToolCallState

	// emitEvent 发出一个 Responses 格式事件帧
	emitEvent := func(eventType string, data map[string]any, u *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: eventType, Data: data, Usage: u})
	}

	// 思考内容：必须落成独立的 reasoning 输出项（output_item.added → summary_part.added →
	// summary 增量 → done 收口）。把 summary 增量挂在 message 项上是协议违规——OpenAI SDK
	// 的流式状态机按 item 类型校验事件归属，错挂会使客户端解析失败并主动断流（client_gone）。
	// Claude 的 interleaved thinking 会出现多个 thinking 块，收口后重开新项。
	var reasoningBuf strings.Builder
	reasoningID := ""
	reasoningItemIdx := -1
	reasoningItems := make([]map[string]any, 0) // 已收口的 reasoning 项，completed 输出数组用
	closeReasoningItem := func() error {
		if reasoningItemIdx < 0 {
			return nil
		}
		text := reasoningBuf.String()
		if err := emitEvent("response.reasoning_summary_text.done", map[string]any{
			"type":          "response.reasoning_summary_text.done",
			"item_id":       reasoningID,
			"output_index":  reasoningItemIdx,
			"summary_index": 0,
			"text":          text,
		}, nil); err != nil {
			return err
		}
		if err := emitEvent("response.reasoning_summary_part.done", map[string]any{
			"type":          "response.reasoning_summary_part.done",
			"item_id":       reasoningID,
			"output_index":  reasoningItemIdx,
			"summary_index": 0,
			"part": map[string]any{
				"type": "summary_text",
				"text": text,
			},
		}, nil); err != nil {
			return err
		}
		item := map[string]any{
			"type":   "reasoning",
			"id":     reasoningID,
			"status": "completed",
			"summary": []map[string]any{{
				"type": "summary_text",
				"text": text,
			}},
		}
		// 客户端 SDK 只回传带 encrypted_content 的 reasoning 项（无状态多轮机制），
		// 网关自编自解，供下一轮还原思考文本（与 chat→responses 桥接口径一致）
		if enc := shared.EncodeReasoningEncryptedContent(text); enc != "" {
			item["encrypted_content"] = enc
		}
		if err := emitEvent("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": reasoningItemIdx,
			"item":         item,
		}, nil); err != nil {
			return err
		}
		reasoningItems = append(reasoningItems, item)
		reasoningItemIdx = -1
		reasoningID = ""
		reasoningBuf.Reset()
		return nil
	}

	// closeTextPart 关闭文本 content part（进入工具调用或流结束时调用）；message 项固定占 output_index 0
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		finishedText := textBuf.String()
		if err := emitEvent("response.output_text.done", map[string]any{
			"type":          "response.output_text.done",
			"item_id":       msgID,
			"output_index":  0,
			"content_index": contentIndex,
			"text":          finishedText,
		}, nil); err != nil {
			return err
		}
		if err := emitEvent("response.content_part.done", map[string]any{
			"type":          "response.content_part.done",
			"item_id":       msgID,
			"output_index":  0,
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
			"output_index": 0,
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
		return nil
	}

	// finish 发送每个工具调用的收尾事件 + response.completed（随帧携带计费用量）
	finish := func() error {
		if sentCompleted {
			return nil
		}
		if err := closeReasoningItem(); err != nil {
			return err
		}
		if err := closeTextPart(); err != nil {
			return err
		}

		// 输出数组：文本消息（closeTextPart 已置 sentTextDone，此处等价于有文本才保留，
		// 与 chat→responses 桥接的 finalOutput 构建口径一致）+ reasoning 项 + 工具调用
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
		// reasoning 项：completed 的 output 数组需携带完整项，客户端据此重建下一轮 input 历史
		finalOutput = append(finalOutput, reasoningItems...)

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

		// 服务端 web_search 调用收尾：web_search_call 项（query 从聚合参数解析）
		for _, st := range serverToolCalls {
			item := claudeWebSearchCallItem(st.id, st.args.String())
			if err := emitEvent("response.output_item.done", map[string]any{
				"type":         "response.output_item.done",
				"output_index": serverToolIndexByID[st.id],
				"item":         item,
			}, nil); err != nil {
				return err
			}
			finalOutput = append(finalOutput, item)
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
				"output_index": 0,
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
				"output_index":  0,
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
				// 先收口 reasoning 项与文本 content part，再开 function_call 项
				if err := closeReasoningItem(); err != nil {
					return err
				}
				if err := closeTextPart(); err != nil {
					return err
				}
				tc := &claudeToolCallState{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
				toolCalls = append(toolCalls, tc)
				toolIndexByID[tc.id] = itemIdx
				currentTool = tc
				if err := emitEvent("response.output_item.added", map[string]any{
					"type":         "response.output_item.added",
					"output_index": itemIdx,
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
				itemIdx++
			case "server_tool_use":
				// claude 服务端工具：仅 web_search 有 Responses 对应物（web_search_call），
				// 其余服务端工具跳过。参数以 input_json_delta 聚合，收尾在 finish 统一冲刷
				if event.ContentBlock.Name != "web_search" {
					continue
				}
				if err := closeReasoningItem(); err != nil {
					return err
				}
				if err := closeTextPart(); err != nil {
					return err
				}
				st := &claudeToolCallState{id: event.ContentBlock.ID, name: event.ContentBlock.Name}
				serverToolCalls = append(serverToolCalls, st)
				serverToolIndexByID[st.id] = itemIdx
				currentServerTool = st
				if err := emitEvent("response.output_item.added", map[string]any{
					"type":         "response.output_item.added",
					"output_index": itemIdx,
					"item": map[string]any{
						"type":   "web_search_call",
						"id":     st.id,
						"status": "in_progress",
					},
				}, nil); err != nil {
					return err
				}
				itemIdx++
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
					// 懒开 reasoning 输出项：首个增量到达时发 output_item.added +
					// summary_part.added，后续增量挂在该项的 id / output_index 上
					if reasoningItemIdx < 0 {
						reasoningItemIdx = itemIdx
						itemIdx++
						reasoningID = fmt.Sprintf("rs_%s_%d", respID, reasoningItemIdx)
						if err := emitEvent("response.output_item.added", map[string]any{
							"type":         "response.output_item.added",
							"output_index": reasoningItemIdx,
							"item": map[string]any{
								"type":    "reasoning",
								"id":      reasoningID,
								"status":  "in_progress",
								"summary": []any{},
							},
						}, nil); err != nil {
							return err
						}
						if err := emitEvent("response.reasoning_summary_part.added", map[string]any{
							"type":          "response.reasoning_summary_part.added",
							"item_id":       reasoningID,
							"output_index":  reasoningItemIdx,
							"summary_index": 0,
							"part": map[string]any{
								"type": "summary_text",
								"text": "",
							},
						}, nil); err != nil {
							return err
						}
					}
					reasoningBuf.WriteString(*event.Delta.Thinking)
					if err := emitEvent("response.reasoning_summary_text.delta", map[string]any{
						"type":          "response.reasoning_summary_text.delta",
						"item_id":       reasoningID,
						"output_index":  reasoningItemIdx,
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
				} else if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" && currentServerTool != nil {
					// web_search_call 无参数增量事件形态，仅聚合（query 收尾时解析）
					currentServerTool.args.WriteString(*event.Delta.PartialJSON)
				}
			case "signature_delta":
				// 思考签名无 Responses 对应物，忽略
			}

		case "content_block_stop":
			// 块级收尾统一延迟到 message_stop / finish，这里仅结束当前工具块的增量定向
			if currentTool != nil {
				currentTool = nil
			}
			if currentServerTool != nil {
				currentServerTool = nil
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
