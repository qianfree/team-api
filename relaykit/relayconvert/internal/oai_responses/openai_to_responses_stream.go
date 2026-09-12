package oai_responses

import (
	"bufio"
	"bytes"
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

// OpenAIToResponsesStreamConverter 将 Chat Completions SSE 流转换为 Responses API 事件流
// （Responses 入站 → chat 上游方向的流式响应合成）。
// 每帧输出 *relayconvert.StreamEvent{Event: "response.xxx", Data: 负载}（事件帧），
// 最终 usage 挂在 response.completed 事件的 StreamEvent.Usage 上（宿主由此提取计费用量）。
type OpenAIToResponsesStreamConverter struct{}

func (c *OpenAIToResponsesStreamConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToOAIResponsesStream
}

func (c *OpenAIToResponsesStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToResponsesStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAIResponses
}

func (c *OpenAIToResponsesStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Chat Completions SSE 流转换为 Responses API SSE 事件流。
// reader 提供上游 chat SSE，转换后的事件经 chunkWriter 以 *relayconvert.StreamEvent 写出。
func (c *OpenAIToResponsesStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}
	echo := extractResponsesRequestEcho(info)
	respID := fmt.Sprintf("resp_%d", time.Now().UnixNano())
	msgID := fmt.Sprintf("msg_%d", time.Now().UnixNano())
	createdAt := int(time.Now().Unix())

	emit := func(event string, data map[string]any, usage *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: event, Data: data, Usage: usage})
	}

	var usage dto.UsageWithDetails
	var contentBuilder strings.Builder
	sentCreated := false
	sentTextDone := false
	parsedChunks := 0
	sawChoices := false
	outputIndex := 0 // message 项固定占用 0，文本事件始终挂在这里
	itemIdx := 1     // 后续输出项（reasoning / tool call）的索引分配器
	contentIndex := 0
	toolCallIndexByID := make(map[string]int)
	toolCallArgsByID := make(map[string]string)
	toolCallNameByID := make(map[string]string)
	// 通过 index 追踪 tool call ID（OpenAI 流式中后续 chunk 的 ID 为空，只有 index）
	toolCallIDByIndex := make(map[int]string)

	// 思考内容：chat 的 reasoning_content 增量必须落成独立的 reasoning 输出项
	// （output_item.added / .done + summary 文本），客户端才能在下一轮把它放进
	// input 回传——只发孤儿 delta 的话历史里永远没有 reasoning 项，
	// DeepSeek 等要求回传 reasoning_content 的 thinking 上游会直接 400。
	var reasoningText strings.Builder
	reasoningID := ""
	reasoningItemIdx := -1
	closeReasoningItem := func() error {
		if reasoningItemIdx < 0 {
			return nil
		}
		text := reasoningText.String()
		if err := emit("response.reasoning_summary_text.done", map[string]any{
			"type":          "response.reasoning_summary_text.done",
			"item_id":       reasoningID,
			"output_index":  reasoningItemIdx,
			"summary_index": 0,
			"text":          text,
		}, nil); err != nil {
			return err
		}
		return emit("response.output_item.done", map[string]any{
			"type":         "response.output_item.done",
			"output_index": reasoningItemIdx,
			"item": map[string]any{
				"type":   "reasoning",
				"id":     reasoningID,
				"status": "completed",
				"summary": []map[string]any{{
					"type": "summary_text",
					"text": text,
				}},
			},
		}, nil)
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
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

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		if data == "" {
			continue
		}

		// 检测 SSE 流中内嵌的上游错误对象：部分聚合商出错时返回 HTTP 200 + SSE，
		// 错误信息夹在 data 行里（{"error":{...}}）。不识别会被当作解析失败静默丢弃，
		// 最终合成空的 response.completed（客户端表现为"成功但无内容"）。
		// 错误的透传/回写由宿主负责，此处直接终止转换。
		if errBody, ok := extractStreamEmbeddedError([]byte(data)); ok {
			return fmt.Errorf("upstream embedded error in SSE stream: %.500s", string(errBody))
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			// 解析失败不中断流（上游可能夹杂非 chat 格式行），后置的协议守卫兜底
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
			if chunk.Model != "" && !isModelMapped(info) {
				modelName = chunk.Model
			}

			if err := emit("response.created", map[string]any{
				"type":     "response.created",
				"response": buildResponsesObjectMap(respID, createdAt, "in_progress", modelName, []any{}, nil, nil, echo),
			}, nil); err != nil {
				return err
			}

			if err := emit("response.output_item.added", map[string]any{
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

			if err := emit("response.content_part.added", map[string]any{
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
		}

		// 提取 usage（含 token 明细，透传给完成事件）
		if chunk.Usage != nil {
			usage.PromptTokens = chunk.Usage.PromptTokens
			usage.CompletionTokens = chunk.Usage.CompletionTokens
			usage.TotalTokens = chunk.Usage.TotalTokens
			usage.PromptTokensDetails = chunk.Usage.PromptTokensDetails
			usage.CompletionTokenDetails = chunk.Usage.CompletionTokenDetails
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
					if err := emit("response.output_text.delta", map[string]any{
						"type":          "response.output_text.delta",
						"item_id":       msgID,
						"output_index":  outputIndex,
						"content_index": contentIndex,
						"delta":         deltaText,
					}, nil); err != nil {
						return err
					}
				}
			}

			// 推理内容：懒开一个 reasoning 输出项（首个增量到达时），
			// 后续增量以该项的 id / output_index 挂载
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				if reasoningItemIdx < 0 {
					reasoningItemIdx = itemIdx
					reasoningID = fmt.Sprintf("rs_%s", respID)
					itemIdx++
					if err := emit("response.output_item.added", map[string]any{
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
				}
				reasoningText.WriteString(*choice.Delta.ReasoningContent)
				if err := emit("response.reasoning_summary_text.delta", map[string]any{
					"type":          "response.reasoning_summary_text.delta",
					"item_id":       reasoningID,
					"output_index":  reasoningItemIdx,
					"summary_index": 0,
					"delta":         *choice.Delta.ReasoningContent,
				}, nil); err != nil {
					return err
				}
			}

			// 工具调用
			for _, tc := range choice.Delta.ToolCalls {
				callID := tc.ID

				// 新 tool call：有 ID 和 name
				if callID != "" && tc.Function.Name != "" {
					// 前一项若是 reasoning，先收口再开工具项
					if err := closeReasoningItem(); err != nil {
						return err
					}
					// 记录 index → callID 映射，用于后续参数 chunk 的查找
					toolCallIDByIndex[tc.Index] = callID

					// 先关闭文本 content part
					if !sentTextDone {
						finishedText := contentBuilder.String()
						if err := emitTextDoneEvents(emit, msgID, outputIndex, contentIndex, finishedText); err != nil {
							return err
						}
						sentTextDone = true
					}

					toolCallIndexByID[callID] = itemIdx
					toolCallNameByID[callID] = tc.Function.Name
					toolCallArgsByID[callID] = ""

					if err := emit("response.output_item.added", map[string]any{
						"type":         "response.output_item.added",
						"output_index": itemIdx,
						"item": map[string]any{
							"type":    "function_call",
							"id":      callID,
							"call_id": callID,
							"name":    tc.Function.Name,
							"status":  "in_progress",
						},
					}, nil); err != nil {
						return err
					}
					itemIdx++
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
					if err := emit("response.function_call_arguments.delta", map[string]any{
						"type":         "response.function_call_arguments.delta",
						"item_id":      callID,
						"output_index": toolCallIndexByID[callID],
						"delta":        tc.Function.Arguments,
					}, nil); err != nil {
						return err
					}
				}
			}

			// finish_reason
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				if err := closeReasoningItem(); err != nil {
					return err
				}
				finishedText := contentBuilder.String()

				// 关闭文本 content part（如果尚未关闭）
				if !sentTextDone {
					if err := emitTextDoneEvents(emit, msgID, outputIndex, contentIndex, finishedText); err != nil {
						return err
					}
				}

				// 发送每个 tool call 的 function_call_arguments.done + output_item.done
				for tcID, tcIdx := range toolCallIndexByID {
					if err := emit("response.function_call_arguments.done", map[string]any{
						"type":         "response.function_call_arguments.done",
						"item_id":      tcID,
						"output_index": tcIdx,
						"arguments":    toolCallArgsByID[tcID],
					}, nil); err != nil {
						return err
					}
					if err := emit("response.output_item.done", map[string]any{
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
					}, nil); err != nil {
						return err
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 上游流始终不是 chat 格式（全程无 choices 且无内容/工具调用/usage）时不静默合成空响应：
	// 典型形态包括 Responses/Claude 格式 SSE（JSON 能解析为空 chat chunk）、非流式 JSON 体、
	// 空 body 等。假成功的空 response.completed 会让客户端"成功但无内容"，必须报错由宿主
	// 驱动重试换渠道/健康上报。
	if !sawChoices && contentBuilder.Len() == 0 && len(toolCallIndexByID) == 0 && usage.TotalTokens == 0 {
		return fmt.Errorf("upstream stream is not chat completions format: %d chunks parsed, none contained choices", parsedChunks)
	}

	// 估算 usage（4 字符/token；流中断的保守估算兜底由宿主处理）
	if usage.CompletionTokens == 0 {
		text := contentBuilder.String()
		if len(text) > 0 {
			usage.CompletionTokens = len(text) / 4
		}
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	// 构建 response.completed 的 output 数组（包含文本消息 + reasoning + 所有 tool call）
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
	// reasoning 项（流中途异常未收口时的兜底；正常路径 closeReasoningItem 已发过 done 事件，
	// completed 的 output 数组仍需携带完整项，客户端据此重建下一轮的 input 历史）
	if reasoningItemIdx >= 0 {
		finalOutput = append(finalOutput, map[string]any{
			"type":   "reasoning",
			"id":     reasoningID,
			"status": "completed",
			"summary": []map[string]any{{
				"type": "summary_text",
				"text": reasoningText.String(),
			}},
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

	// response.completed（最终 usage 同时挂在 StreamEvent.Usage 上供宿主提取）
	completedAt := int(time.Now().Unix())
	finalUsage := usage
	return emit("response.completed", map[string]any{
		"type":     "response.completed",
		"response": buildResponsesObjectMap(respID, createdAt, "completed", modelName, finalOutput, buildResponsesUsageMap(&usage), &completedAt, echo),
	}, &finalUsage)
}

// emitTextDoneEvents 关闭文本 content part：依次发送 response.output_text.done、
// response.content_part.done、response.output_item.done（message completed）。
func emitTextDoneEvents(emit func(string, map[string]any, *dto.UsageWithDetails) error, msgID string, outputIndex int, contentIndex int, finishedText string) error {
	if err := emit("response.output_text.done", map[string]any{
		"type":          "response.output_text.done",
		"item_id":       msgID,
		"output_index":  outputIndex,
		"content_index": contentIndex,
		"text":          finishedText,
	}, nil); err != nil {
		return err
	}
	if err := emit("response.content_part.done", map[string]any{
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
	return emit("response.output_item.done", map[string]any{
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
	}, nil)
}

// extractStreamEmbeddedError 检测 SSE data 行中内嵌的上游错误对象（存在 "error" 键且值非 null）。
// 对流式 chunk 逐行检测，并排除 "error":null（部分供应商的正常 chunk 会携带空 error 字段）。
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

// buildResponsesObjectMap 构建 Responses API response 对象的完整字段 map
// （私有等价实现，宿主导出版 BuildResponsesObjectMap 仍被其他适配器复用）。
// 请求参数从 echo 快照回显（快照缺失时回退默认值）；
// store 恒为 false——合成响应不落上游存储，客户端不可经生命周期端点 retrieve。
func buildResponsesObjectMap(respID string, createdAt int, status string, model string, output any, usageObj map[string]any, completedAt *int, echo responsesRequestEcho) map[string]any {
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
// （私有等价实现，宿主导出版 BuildResponsesUsageMap 仍被其他适配器复用）。
func buildResponsesUsageMap(usage *dto.UsageWithDetails) map[string]any {
	inputDetails := map[string]any{"cached_tokens": 0}
	outputDetails := map[string]any{"reasoning_tokens": 0}
	if usage.PromptTokensDetails != nil {
		inputDetails = map[string]any{
			"cached_tokens":      usage.PromptTokensDetails.CachedTokens,
			"cache_write_tokens": usage.PromptTokensDetails.CacheWriteTokens,
			"audio_tokens":       usage.PromptTokensDetails.AudioTokens,
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
