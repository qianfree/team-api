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

// geminiResponsesToolCall Responses 桥接中聚合的工具调用（Gemini functionCall）
type geminiResponsesToolCall struct {
	id   string
	name string
	args string
}

// GeminiToResponsesStreamConverter 将 Gemini SSE 流转换为 OpenAI Responses 事件流。
// 事件序列与 chat→responses、claude→responses 两条既有桥接保持一致
// （response.created → output_item/content_part 增量 → 各项 done → response.completed），
// 每个事件以 StreamEvent{Event: 事件名} 输出，由宿主桥接层写为事件帧。
type GeminiToResponsesStreamConverter struct{}

func (c *GeminiToResponsesStreamConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToOAIResponsesStream
}

func (c *GeminiToResponsesStreamConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToResponsesStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *GeminiToResponsesStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Gemini SSE 流转换为 Responses 事件帧序列。
// reader 提供上游 Gemini 的原生 SSE（Code Assist 包装的解包在宿主侧完成）。
func (c *GeminiToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// 响应/消息 ID 基准：宿主 RequestID 未随 Meta 下沉，以纳秒时间戳保证单响应内唯一
	idBase := fmt.Sprintf("%d", time.Now().UnixNano())
	respID := fmt.Sprintf("resp_%s", idBase)
	msgID := fmt.Sprintf("msg_%s", idBase)
	createdAt := int(time.Now().Unix())
	modelName := originModelName(info)

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

	// emitEvent 发出一个 Responses 格式事件帧
	emitEvent := func(eventType string, data map[string]any, usage *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: eventType, Data: data, Usage: usage})
	}

	// ensureCreated 发出 response.created 与首个 message 项的开场事件
	ensureCreated := func() error {
		if sentCreated {
			return nil
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
		return nil
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

	// finish 发送各工具调用的收尾事件与 response.completed（随帧携带计费用量）
	finish := func() error {
		if sentCompleted {
			return nil
		}
		if err := ensureCreated(); err != nil {
			return err
		}
		if err := closeTextPart(); err != nil {
			return err
		}

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
				"arguments":    tc.args,
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
					"arguments": tc.args,
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
				"arguments": tc.args,
				"status":    "completed",
			})
		}

		// 本方向客户端可见 usage 与计费口径一致（OpenAI 语义），completed 帧同时携带计费用量；
		// 输出缺失时的估算兜底（EstimateStreamOutputTokens）由宿主桥接层处理
		usage := geminiUsageToDetails(&totalUsage)
		completedAt := int(time.Now().Unix())
		if err := emitEvent("response.completed", map[string]any{
			"type": "response.completed",
			"response": buildResponsesObjectMap(respID, createdAt, "completed", modelName,
				finalOutput, buildResponsesUsageMap(usage), &completedAt, info),
		}, usage); err != nil {
			return err
		}
		sentCompleted = true
		return nil
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			// 流中断：计费兜底（Gemini 每 chunk 携带累计 usage + 请求侧估算）由宿主桥接层处理
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			break
		}

		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
			continue
		}

		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}
		if geminiResp.ModelName != "" && !isModelMapped(info) {
			modelName = geminiResp.ModelName
		}

		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			// 事件流已开场，必须补齐 completed 事件，否则客户端挂起
			if err := finish(); err != nil {
				return err
			}
			return fmt.Errorf("request blocked by Gemini safety filter: %s: %w", geminiResp.PromptFeedback.BlockReason, relayconvert.ErrContentBlocked)
		}

		if err := ensureCreated(); err != nil {
			return err
		}

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
						if err := emitEvent("response.reasoning_summary_text.delta", map[string]any{
							"type":          "response.reasoning_summary_text.delta",
							"item_id":       msgID,
							"output_index":  0,
							"summary_index": 0,
							"delta":         part.Text,
						}, nil); err != nil {
							return err
						}
					} else {
						textBuf.WriteString(part.Text)
						if err := emitEvent("response.output_text.delta", map[string]any{
							"type":          "response.output_text.delta",
							"item_id":       msgID,
							"output_index":  0,
							"content_index": contentIndex,
							"delta":         part.Text,
						}, nil); err != nil {
							return err
						}
					}
				}

				if part.FunctionCall != nil {
					// 先关闭文本 content part，再开 function_call 项
					if err := closeTextPart(); err != nil {
						return err
					}

					tc := &geminiResponsesToolCall{
						id:   geminiResponsesCallID(part.FunctionCall, idBase, toolIdx),
						name: part.FunctionCall.FunctionName,
						args: marshalGeminiArgs(part.FunctionCall),
					}
					toolIdx++
					toolCalls = append(toolCalls, tc)
					toolIndexByID[tc.id] = outputIndex

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
					// Gemini 一次给全量参数，单条 delta 发完（收尾的 .done 在 finish 中统一发）
					if err := emitEvent("response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      tc.id,
						"output_index": outputIndex,
						"delta":        tc.args,
					}, nil); err != nil {
						return err
					}
					outputIndex++
				}

				// Responses 的 output_text 无对应块类型，按既有 Gemini 出站口径渲染为文本
				if fallback := geminiPartFallbackText(part); fallback != "" {
					textBuf.WriteString(fallback)
					if err := emitEvent("response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  0,
						"content_index": contentIndex,
						"delta":         fallback,
					}, nil); err != nil {
						return err
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		// 读流出错：已累计的 usage 无法随错误返回（StreamEvent 只随帧携带），由宿主按估算兜底
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游断流未给收尾：仍合成 completed，避免客户端挂起
	return finish()
}
