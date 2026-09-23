package oai_chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeStreamConverter 将 OpenAI Chat Completions SSE 流转换为 Claude Messages 事件流。
// 每个 Claude 事件通过 chunkWriter 以 *relayconvert.StreamEvent 封装输出
// （Event=事件名，Data=事件负载；带 usage 的事件同时填 Usage 供宿主捕获用量）。
type OpenAIToClaudeStreamConverter struct{}

func (c *OpenAIToClaudeStreamConverter) ID() string {
	return relayconvert.ResponseConverterOAIChatToClaudeMessagesStream
}

func (c *OpenAIToClaudeStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeStreamConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *OpenAIToClaudeStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 读取上游 OpenAI SSE 流（data: {...} 行 + data: [DONE] 结束），
// 转换为 Claude 事件序列：message_start / content_block_start / content_block_delta /
// content_block_stop / message_delta / message_stop。
// 宿主侧职责（SSE 帧化、ping、首字节时间、流中断结算）不在此处理。
func (c *OpenAIToClaudeStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	msgID := generateClaudeMessageID()
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
		if isModelMapped(info) {
			modelName = info.GetUpstreamModelName()
		}
	}

	// writeEvent 输出一帧 Claude 事件（usage 仅在需要宿主捕获用量的事件上携带）
	writeEvent := func(eventType string, data *dto.ClaudeResponse, usage *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: eventType, Data: data, Usage: usage})
	}

	var (
		startSent         bool
		doneSent          bool
		finishReason      string
		contentIndex      int
		inputTokens       int
		outputTokens      int
		cachedTokens      int               // prompt_tokens_details.cached_tokens（OpenAI 口径：已含于 prompt_tokens）
		promptDetails     *dto.TokenDetails // 缓存等输入明细，透传给宿主计费（按明细扣减缓存价）
		completionDetails *dto.TokenDetails // 思考等输出明细，透传给宿主计费
		currentBlockType  string            // 跟踪当前 block 类型: "text" / "thinking" / "tool_use"
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
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

		if data == "[DONE]" {
			// 关闭当前 content block（如果有的话）
			if currentBlockType != "" {
				blockStop := dto.ClaudeResponse{
					Type:  "content_block_stop",
					Index: intPtr(contentIndex),
				}
				if err := writeEvent("content_block_stop", &blockStop, nil); err != nil {
					return err
				}
				contentIndex++
				currentBlockType = ""
			}

			// 发送 message_delta（stop_reason）+ message_stop。
			// Claude 语义：input_tokens 不含 cache_read（OpenAI 的 prompt_tokens 含 cached，扣减后映射）；
			// OpenAI 的 usage 在流尾才返回、message_start 时不可得，最终用量在此补报
			reason := mapOpenAIFinishReasonToClaude(finishReason)
			deltaInput := inputTokens - cachedTokens
			if deltaInput < 0 {
				deltaInput = 0
			}
			delta := dto.ClaudeResponse{
				Type: "message_delta",
				Delta: &dto.ClaudeDelta{
					StopReason: strPtr(reason),
				},
				Usage: &dto.ClaudeUsage{
					InputTokens:          deltaInput,
					CacheReadInputTokens: cachedTokens,
					OutputTokens:         outputTokens,
				},
			}
			// 宿主捕获的用量保持 OpenAI 原始口径（prompt 含 cached）并携带明细，
			// 缓存扣减由宿主计费侧按明细处理，避免缓存按 input 全价计费
			if err := writeEvent("message_delta", &delta, &dto.UsageWithDetails{
				PromptTokens:           inputTokens,
				CompletionTokens:       outputTokens,
				TotalTokens:            inputTokens + outputTokens,
				PromptTokensDetails:    promptDetails,
				CompletionTokenDetails: completionDetails,
			}); err != nil {
				return err
			}

			stopEvent := dto.ClaudeResponse{Type: "message_stop"}
			if err := writeEvent("message_stop", &stopEvent, nil); err != nil {
				return err
			}

			doneSent = true
			break
		}

		var streamResp dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		// 提取 usage（OpenAI usage 通常在最后一个 chunk）
		if streamResp.Usage != nil {
			inputTokens = streamResp.Usage.PromptTokens
			outputTokens = streamResp.Usage.CompletionTokens
			if streamResp.Usage.PromptTokensDetails != nil {
				cachedTokens = streamResp.Usage.PromptTokensDetails.CachedTokens
			}
			// 缓存/思考明细透传给宿主计费；OpenAI 的 prompt_tokens 含 cached（子集语义），
			// 宿主计费按明细扣减缓存部分，避免缓存按 input 全价计费
			promptDetails = streamResp.Usage.PromptTokensDetails
			completionDetails = streamResp.Usage.CompletionTokenDetails
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
			if err := writeEvent("message_start", &startEvent, nil); err != nil {
				return err
			}
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
					if err := writeEvent("content_block_stop", &blockStop, nil); err != nil {
						return err
					}
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
					if err := writeEvent("content_block_start", &blockStart, nil); err != nil {
						return err
					}
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
				if err := writeEvent("content_block_delta", &delta, nil); err != nil {
					return err
				}
			}

			// 推理内容（thinking）
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				// 如果当前 block 不是 thinking，先关闭前一个 block
				if currentBlockType != "" && currentBlockType != "thinking" {
					blockStop := dto.ClaudeResponse{
						Type:  "content_block_stop",
						Index: intPtr(contentIndex),
					}
					if err := writeEvent("content_block_stop", &blockStop, nil); err != nil {
						return err
					}
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
					if err := writeEvent("content_block_start", &blockStart, nil); err != nil {
						return err
					}
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
				if err := writeEvent("content_block_delta", &delta, nil); err != nil {
					return err
				}
			}

			// 工具调用
			for _, tc := range choice.Delta.ToolCalls {
				// content_block_start 用于 tool_use（仅在 function name 出现时）
				if tc.Function.Name != "" {
					// 先关闭前一个 block
					if currentBlockType != "" {
						blockStop := dto.ClaudeResponse{
							Type:  "content_block_stop",
							Index: intPtr(contentIndex),
						}
						if err := writeEvent("content_block_stop", &blockStop, nil); err != nil {
							return err
						}
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
					if err := writeEvent("content_block_start", &blockStart, nil); err != nil {
						return err
					}
					currentBlockType = "tool_use"
				}

				// content_block_delta 用于工具参数
				if tc.Function.Arguments != "" {
					args := tc.Function.Arguments
					delta := dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{
							Type:        "input_json_delta",
							PartialJSON: &args,
						},
					}
					if err := writeEvent("content_block_delta", &delta, nil); err != nil {
						return err
					}
				}
			}

			// finish_reason
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	if !doneSent {
		// 流意外结束（未收到 [DONE]），发送终止事件；
		// 已收到的 usage 附在 message_stop 上供宿主捕获，避免整段按零值计费漏计
		if currentBlockType != "" {
			blockStop := dto.ClaudeResponse{
				Type:  "content_block_stop",
				Index: intPtr(contentIndex),
			}
			if err := writeEvent("content_block_stop", &blockStop, nil); err != nil {
				return err
			}
		}
		stopEvent := dto.ClaudeResponse{Type: "message_stop"}
		if err := writeEvent("message_stop", &stopEvent, &dto.UsageWithDetails{
			PromptTokens:           inputTokens,
			CompletionTokens:       outputTokens,
			TotalTokens:            inputTokens + outputTokens,
			PromptTokensDetails:    promptDetails,
			CompletionTokenDetails: completionDetails,
		}); err != nil {
			return err
		}
	}

	return nil
}

// intPtr 返回 int 的指针
func intPtr(v int) *int {
	return &v
}

// 转换器在宿主应用的包初始化阶段注册，而非在此 internal 实现包中完成。
