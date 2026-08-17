package oai_responses

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIChatToResponsesStreamConverter OpenAI Chat 上游 SSE → Responses 客户端 SSE（流式响应侧）。
// 移植自宿主 relay/channel/openai/responses.go 的 handleResponsesInboundStream 状态机，
// 经流式注册表按 (openai→responses) 路由（codex 打 chat-only 渠道的流式主路径）。
//
// 与 legacy 的确定性差异（顺手修复项，golden/单测锁定）：
//   - 多工具的 done 事件与 completed output 数组按登记顺序（legacy 遍历 map 顺序随机）；
//   - 重复 finish_reason 不再重复发 done（legacy 无去重标志）；
//   - completedAt = max(NowFunc, createdAt)（legacy 为独立 Now()，可能出现 completed < created）。
//
// 错误契约（宿主桥接经 errors.Is/As 分类）：
//   - SSE 内嵌上游错误 → *types.EmbeddedUpstreamError（原文载荷）；
//   - 上游流非 chat 格式（假成功防护）→ 包装 types.ErrProtocolMismatch 的错误；
//   - 客户端断开 → ctx.Err()。
type OpenAIChatToResponsesStreamConverter struct{}

// chatToolCallState 流式聚合的工具调用；tools slice 按登记顺序是 done 事件与
// finalOutput 的唯一遍历源（替代 legacy 的 map 遍历，保证顺序确定）。
type chatToolCallState struct {
	id    string
	name  string
	args  strings.Builder
	index int  // 登记时分配的 output_index
	done  bool // 已发 function_call_arguments.done + output_item.done（去重）
}

// ConvertStreamResponse 读取 chat SSE reader，经 chunkWriter 输出 *dto.ResponsesStreamEvent。
func (c *OpenAIChatToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}
	respID := fmt.Sprintf("resp_%d", NowFunc().UnixNano())
	msgID := fmt.Sprintf("msg_%d", NowFunc().UnixNano())
	createdAt := int(NowFunc().Unix())

	var usage dto.UsageWithDetails
	var contentBuilder strings.Builder
	sentCreated := false
	sentTextDone := false
	parsedChunks := 0
	sawChoices := false
	outputIndex := 0
	contentIndex := 0
	tools := make([]*chatToolCallState, 0)
	toolByID := make(map[string]*chatToolCallState)
	// OpenAI 流式中后续参数 chunk 的 ID 为空只有 index，靠 index 反查
	toolIDByUpstreamIndex := make(map[int]string)
	echo := responsesEchoOf(info)

	// emit 输出一个 Responses SSE 事件（chunkWriter 出错即客户端写失败，终止转换）
	emit := func(eventType string, payload map[string]any) error {
		payload["type"] = eventType
		return chunkWriter(&dto.ResponsesStreamEvent{Type: eventType, Data: payload})
	}

	// closeTextPart 关闭文本 content part（进入工具调用或 finish 时调用）
	closeTextPart := func() error {
		if sentTextDone {
			return nil
		}
		finishedText := contentBuilder.String()
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

	// finishTools 按登记顺序为每个未收尾的工具发 done 双事件（done 标志兼作去重）
	finishTools := func() error {
		for _, tool := range tools {
			if tool.done {
				continue
			}
			if err := emit("response.function_call_arguments.done", map[string]any{
				"item_id":      tool.id,
				"output_index": tool.index,
				"arguments":    tool.args.String(),
			}); err != nil {
				return err
			}
			if err := emit("response.output_item.done", map[string]any{
				"output_index": tool.index,
				"item": map[string]any{
					"type":      "function_call",
					"id":        tool.id,
					"call_id":   tool.id,
					"name":      tool.name,
					"arguments": tool.args.String(),
					"status":    "completed",
				},
			}); err != nil {
				return err
			}
			tool.done = true
		}
		return nil
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractSSEData(line)
		if data == "[DONE]" {
			break
		}

		// SSE 流中内嵌的上游错误对象：部分聚合商出错时返回 HTTP 200 + SSE，
		// 错误信息夹在 data 行里。不识别会合成空的 response.completed（客户端"成功但无内容"）
		if errBody, ok := extractStreamEmbeddedError([]byte(data)); ok {
			return &types.EmbeddedUpstreamError{Body: errBody}
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		parsedChunks++
		if len(chunk.Choices) > 0 {
			sawChoices = true
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
			if chunk.Model != "" && !modelMappedOf(info) {
				modelName = chunk.Model
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
		}

		// 提取 usage（legacy 口径：仅取三项数值，chunk 内的 usage details 丢弃）
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
		}

		// 处理 choices delta
		for _, choice := range chunk.Choices {
			// 文本内容 delta（仅 string 形态，非 string 静默丢弃——legacy 口径）
			if choice.Delta.Content != nil {
				if deltaText, ok := choice.Delta.Content.(string); ok && deltaText != "" {
					contentBuilder.WriteString(deltaText)
					if err := emit("response.output_text.delta", map[string]any{
						"item_id":       msgID,
						"output_index":  outputIndex,
						"content_index": contentIndex,
						"delta":         deltaText,
					}); err != nil {
						return err
					}
				}
			}

			// 推理内容：仅流中透出，不进 completed 的 output（legacy 口径）
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				if err := emit("response.reasoning_summary_text.delta", map[string]any{
					"item_id":       msgID,
					"output_index":  outputIndex,
					"summary_index": 0,
					"delta":         *choice.Delta.ReasoningContent,
				}); err != nil {
					return err
				}
			}

			// 工具调用增量
			for _, tc := range choice.Delta.ToolCalls {
				callID := tc.ID

				// 新 tool call：有 ID 和 name
				if callID != "" && tc.Function.Name != "" {
					toolIDByUpstreamIndex[tc.Index] = callID
					// 先关闭文本 content part
					if err := closeTextPart(); err != nil {
						return err
					}
					tool := &chatToolCallState{id: callID, name: tc.Function.Name, index: outputIndex}
					tools = append(tools, tool)
					toolByID[callID] = tool

					if err := emit("response.output_item.added", map[string]any{
						"output_index": outputIndex,
						"item": map[string]any{
							"type":    "function_call",
							"id":      callID,
							"call_id": callID,
							"name":    tc.Function.Name,
							"status":  "in_progress",
						},
					}); err != nil {
						return err
					}
					outputIndex++
				}

				// 参数 chunk：ID 可能为空，通过 index 查找对应的 callID
				if callID == "" {
					callID = toolIDByUpstreamIndex[tc.Index]
				}
				if callID == "" {
					continue
				}

				if tc.Function.Arguments != "" {
					// legacy 怪癖保持：未登记的 callID（仅 ID 无 name 的 chunk）参数不进入
					// done 事件与 completed output，仅透传 delta（output_index 取零值）
					toolIdx := 0
					if tool, ok := toolByID[callID]; ok {
						tool.args.WriteString(tc.Function.Arguments)
						toolIdx = tool.index
					}
					if err := emit("response.function_call_arguments.delta", map[string]any{
						"item_id":      callID,
						"output_index": toolIdx,
						"delta":        tc.Function.Arguments,
					}); err != nil {
						return err
					}
				}
			}

			// finish_reason：关闭文本 part + 按登记顺序收尾全部工具
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if err := closeTextPart(); err != nil {
					return err
				}
				if err := finishTools(); err != nil {
					return err
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 假成功防护：上游流始终不是 chat 格式（无 choices 且无内容/工具/usage）时不再
	// 静默合成空响应——报错驱动重试换渠道/健康上报
	if !sawChoices && contentBuilder.Len() == 0 && len(tools) == 0 && usage.TotalTokens == 0 {
		return fmt.Errorf("%w: %d chunks parsed, none contained choices", types.ErrProtocolMismatch, parsedChunks)
	}

	// 客户端可见 usage 兜底（legacy 客户端可见口径：正常结束路径为 len/4 估算）
	if usage.CompletionTokens == 0 && contentBuilder.Len() > 0 {
		usage.CompletionTokens = contentBuilder.Len() / 4
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// 构建 completed 的 output 数组（文本消息 + 工具，按登记顺序）
	finalOutput := make([]dto.ResponsesOutput, 0)
	if !sentTextDone || contentBuilder.Len() > 0 {
		finalOutput = append(finalOutput, dto.ResponsesOutput{
			Type:   "message",
			ID:     msgID,
			Status: "completed",
			Role:   "assistant",
			Content: []dto.ResponsesOutputContent{{
				Type:        "output_text",
				Text:        contentBuilder.String(),
				Annotations: []dto.ResponsesAnnotation{},
			}},
		})
	}
	for _, tool := range tools {
		finalOutput = append(finalOutput, dto.ResponsesOutput{
			Type:      "function_call",
			ID:        tool.id,
			CallID:    tool.id,
			Name:      tool.name,
			Arguments: tool.args.String(),
			Status:    "completed",
		})
	}

	completedAt := int(NowFunc().Unix())
	if completedAt < createdAt {
		completedAt = createdAt
	}
	return emit("response.completed", map[string]any{
		"response": buildResponsesObject(respID, createdAt, "completed", modelName, finalOutput,
			responsesUsageOf(&usage), &completedAt, echo),
	})
}

// extractStreamEmbeddedError 检测 SSE data 行中内嵌的上游错误对象（存在 "error" 键且值非 null）。
// 部分供应商的正常 chunk 会携带空 error 字段，需排除 "error":null。
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
