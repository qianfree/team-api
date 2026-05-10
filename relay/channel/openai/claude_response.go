package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// handleClaudeInboundNonStream 将 OpenAI 非流式响应转换为 Claude 格式
func handleClaudeInboundNonStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// 将上游错误转换为 Claude 格式透传给客户端
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		claudeErr, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "api_error", "message": string(body)},
		})
		_, _ = writer.Write(claudeErr)
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	var openaiResp dto.ChatCompletionResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("invalid response body: %w", err)
	}

	// 转换为 Claude 格式
	claudeResp := openAIToClaudeResponse(&openaiResp, info)

	respBody, _ := json.Marshal(claudeResp)
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(respBody)

	usage := &common.Usage{}
	if len(openaiResp.Choices) > 0 {
		usage.PromptTokens = openaiResp.Usage.PromptTokens
		usage.CompletionTokens = openaiResp.Usage.CompletionTokens
		usage.TotalTokens = openaiResp.Usage.TotalTokens
		usage.PromptTokensDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.PromptTokensDetails)
		usage.CompletionTokenDetails = common.DtoTokenDetailsToCommon(openaiResp.Usage.CompletionTokenDetails)
	}
	usage.CacheIncludedInPrompt = true
	return usage, nil
}

// handleClaudeInboundStream 将 OpenAI SSE 流转换为 Claude SSE 流
func handleClaudeInboundStream(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(resp.StatusCode)
		claudeErr, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "api_error", "message": string(body)},
		})
		_, _ = writer.Write(claudeErr)
		return &common.Usage{}, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	helper.SetEventStreamHeaders(writer)
	defer helper.PingTicker(writer, 15*time.Second)()

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	msgID := fmt.Sprintf("msg_%s", info.RequestID)
	modelName := info.OriginModelName
	if info.ChannelMeta.IsModelMapped {
		modelName = info.ChannelMeta.UpstreamModelName
	}

	var (
		usage            common.Usage
		startSent        bool
		finishReason     string
		contentIndex     int
		inputTokens      int
		outputTokens     int
		currentBlockType string // 跟踪当前 block 类型: "text" 或 "tool_use"
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, ctx.Err())
			return &usage, common.ErrStreamInterrupted
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

		if data != "" && data != "[DONE]" {
			info.SetFirstResponseTime()
		}

		if data == "[DONE]" {
			// 关闭当前 content block（如果有的话）
			if currentBlockType != "" {
				blockStop := dto.ClaudeResponse{
					Type:  "content_block_stop",
					Index: intPtr(contentIndex),
				}
				writeClaudeSSE(writer, "content_block_stop", &blockStop)
				contentIndex++
				currentBlockType = ""
			}

			// 发送 message_delta（stop_reason）+ message_stop
			reason := common.OpenAIFinishReasonToClaude(finishReason)
			delta := dto.ClaudeResponse{
				Type: "message_delta",
				Delta: &dto.ClaudeDelta{
					StopReason: strPtr(reason),
				},
				Usage: &dto.ClaudeUsage{
					OutputTokens: outputTokens,
				},
			}
			writeClaudeSSE(writer, "message_delta", &delta)

			stopEvent := dto.ClaudeResponse{Type: "message_stop"}
			writeClaudeSSE(writer, "message_stop", &stopEvent)

			info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
			break
		}

		var streamResp dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		// 提取 usage
		if streamResp.Usage != nil {
			inputTokens = streamResp.Usage.PromptTokens
			outputTokens = streamResp.Usage.CompletionTokens
		}

		// 发送 message_start（首块数据时）
		if !startSent {
			startEvent := dto.ClaudeResponse{
				Type: "message_start",
				Message: &dto.ClaudeMessageInfo{
					ID:           msgID,
					Type:         "message",
					Role:         "assistant",
					Content:      []dto.ClaudeContentBlock{},
					Model:        modelName,
					StopReason:   nil,
					StopSequence: nil,
					Usage: &dto.ClaudeUsage{
						InputTokens:  inputTokens,
						OutputTokens: 0,
					},
				},
			}
			writeClaudeSSE(writer, "message_start", &startEvent)
			startSent = true
		}

		// 处理每个 choice delta
		for _, choice := range streamResp.Choices {
			// 文本内容
			if text, ok := choice.Delta.Content.(string); ok && text != "" {
				// 如果当前 block 不是 text，先关闭前一个 block
				if currentBlockType != "" && currentBlockType != "text" {
					blockStop := dto.ClaudeResponse{
						Type:  "content_block_stop",
						Index: intPtr(contentIndex),
					}
					writeClaudeSSE(writer, "content_block_stop", &blockStop)
					contentIndex++
				}

				// content_block_start（首次文本输出）
				if currentBlockType != "text" {
					blockStart := dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: intPtr(contentIndex),
						ContentBlock: &dto.ClaudeContentBlock{
							Type: "text",
							Text: strPtr(""),
						},
					}
					writeClaudeSSE(writer, "content_block_start", &blockStart)
					currentBlockType = "text"
				}

				delta := dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: intPtr(contentIndex),
					Delta: &dto.ClaudeDelta{
						Type: "text_delta",
						Text: strPtr(text),
					},
				}
				writeClaudeSSE(writer, "content_block_delta", &delta)
			}

			// reasoning content (thinking)
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				// 如果当前 block 不是 thinking，先关闭前一个 block
				if currentBlockType != "" && currentBlockType != "thinking" {
					blockStop := dto.ClaudeResponse{
						Type:  "content_block_stop",
						Index: intPtr(contentIndex),
					}
					writeClaudeSSE(writer, "content_block_stop", &blockStop)
					contentIndex++
				}

				if currentBlockType != "thinking" {
					blockStart := dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: intPtr(contentIndex),
						ContentBlock: &dto.ClaudeContentBlock{
							Type:     "thinking",
							Thinking: strPtr(""),
						},
					}
					writeClaudeSSE(writer, "content_block_start", &blockStart)
					currentBlockType = "thinking"
				}

				delta := dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: intPtr(contentIndex),
					Delta: &dto.ClaudeDelta{
						Type:     "thinking_delta",
						Thinking: choice.Delta.ReasoningContent,
					},
				}
				writeClaudeSSE(writer, "content_block_delta", &delta)
			}

			// tool calls
			for _, tc := range choice.Delta.ToolCalls {
				// content_block_start for tool_use（仅在 function name 出现时）
				if tc.Function.Name != "" {
					// 先关闭前一个 block
					if currentBlockType != "" {
						blockStop := dto.ClaudeResponse{
							Type:  "content_block_stop",
							Index: intPtr(contentIndex),
						}
						writeClaudeSSE(writer, "content_block_stop", &blockStop)
						contentIndex++
					}

					blockStart := dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: intPtr(contentIndex),
						ContentBlock: &dto.ClaudeContentBlock{
							Type:  "tool_use",
							ID:    tc.ID,
							Name:  tc.Function.Name,
							Input: map[string]any{},
						},
					}
					writeClaudeSSE(writer, "content_block_start", &blockStart)
					currentBlockType = "tool_use"
				}

				// content_block_delta for tool arguments
				if tc.Function.Arguments != "" {
					delta := dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{
							Type:        "input_json_delta",
							PartialJSON: &tc.Function.Arguments,
						},
					}
					writeClaudeSSE(writer, "content_block_delta", &delta)
				}
			}

			// finish_reason
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		info.StreamStatus.SetEndReason(common.StreamEndReasonError, err)
		return &usage, fmt.Errorf("stream scanner error: %w", err)
	}

	if info.StreamStatus.GetEndReason() == "" {
		// 流意外结束，发送终止事件
		if currentBlockType != "" {
			blockStop := dto.ClaudeResponse{
				Type:  "content_block_stop",
				Index: intPtr(contentIndex),
			}
			writeClaudeSSE(writer, "content_block_stop", &blockStop)
		}
		stopEvent := dto.ClaudeResponse{Type: "message_stop"}
		writeClaudeSSE(writer, "message_stop", &stopEvent)
		info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	}

	usage.PromptTokens = inputTokens
	usage.CompletionTokens = outputTokens
	usage.TotalTokens = inputTokens + outputTokens
	usage.CacheIncludedInPrompt = true

	return &usage, nil
}

// openAIToClaudeResponse 将 OpenAI ChatCompletion 响应转换为 Claude Messages 响应
func openAIToClaudeResponse(openaiResp *dto.ChatCompletionResponse, info *common.RelayInfo) dto.ClaudeResponse {
	content := make([]dto.ClaudeContentBlock, 0)
	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ClaudeContentBlock

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]

		// 提取文本
		if text, ok := choice.Message.Content.(string); ok && text != "" {
			textParts = append(textParts, text)
		}

		// 提取思维内容
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			thinkingParts = append(thinkingParts, *choice.Message.ReasoningContent)
		}

		// 提取工具调用
		for _, tc := range choice.Message.ToolCalls {
			var inputObj any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
				inputObj = map[string]any{}
			}
			toolCalls = append(toolCalls, dto.ClaudeContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputObj,
			})
		}
	}

	// 添加思维块（如果有）
	for _, thinking := range thinkingParts {
		content = append(content, dto.ClaudeContentBlock{
			Type:     "thinking",
			Thinking: strPtr(thinking),
		})
	}

	// 添加文本块
	for _, text := range textParts {
		content = append(content, dto.ClaudeContentBlock{
			Type: "text",
			Text: strPtr(text),
		})
	}

	// 添加工具调用块
	content = append(content, toolCalls...)

	if len(content) == 0 {
		content = append(content, dto.ClaudeContentBlock{
			Type: "text",
			Text: strPtr(""),
		})
	}

	modelName := openaiResp.Model
	if info.ChannelMeta.IsModelMapped {
		modelName = info.OriginModelName
	}

	stopReason := "end_turn"
	if len(openaiResp.Choices) > 0 {
		stopReason = common.OpenAIFinishReasonToClaude(openaiResp.Choices[0].FinishReason)
	}

	return dto.ClaudeResponse{
		ID:           fmt.Sprintf("msg_%s", info.RequestID),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		StopReason:   stopReason,
		StopSequence: nil,
		Model:        modelName,
		Usage: &dto.ClaudeUsage{
			InputTokens:          openaiResp.Usage.PromptTokens,
			OutputTokens:         openaiResp.Usage.CompletionTokens,
			CacheReadInputTokens: openaiResp.Usage.PromptTokensDetails.CachedTokens,
		},
	}
}

// writeClaudeSSE 写入 Claude 格式的 SSE 事件
func writeClaudeSSE(w http.ResponseWriter, eventType string, data any) {
	dataJSON, _ := json.Marshal(data)
	_ = helper.WriteSSEEvent(w, eventType, string(dataJSON))
}

// intPtr 返回 int 的指针
func intPtr(v int) *int {
	return &v
}

// strPtr 返回 string 的指针
func strPtr(v string) *string {
	return &v
}
