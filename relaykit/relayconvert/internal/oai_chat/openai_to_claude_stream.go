package oai_chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/reasonmap"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeStreamConverter OpenAI Chat 上游 SSE → Claude 客户端 SSE（流式响应侧，P2）。
// 移植宿主 relay/channel/openai/claude_response.go 的 handleClaudeInboundStream 状态机，
// chunk 输出 *dto.ClaudeStreamEvent（宿主写出 `event:`+`data:` 格式，message_stop 收尾无 [DONE]）。
//
// 与 legacy 的确定性差异（顺手修复项，单测锁定）：
//   - tool_calls 参数 delta 按 tc.Index 反查所属块（legacy 用当前 contentIndex，多工具
//     交错且参数片段晚到时会错挂块——P1-R openai_chat_to_responses_stream 的反查蓝本）；
//   - 意外断流（无 [DONE] 的 EOF）收尾补发 message_delta（legacy 只发 message_stop，
//     客户端拿不到 stop_reason 与最终 usage）。
//
// legacy 语义保持：流式模型名与非流式方向相反（默认 OriginModelName、映射渠道→UpstreamModelName）；
// thinking delta 复用指针；message_delta 的 usage 为 Claude 扣减口径（Input=Prompt-Cached）。
type OpenAIToClaudeStreamConverter struct{}

func (c *OpenAIToClaudeStreamConverter) ID() string {
	// 独立流式转换器：ID/From/To 表达自身真实流方向（openai→claude），
	// 与挂载 spec 的 Resp 侧方向约定（spec From/To=请求方向、Resp 反向）无关
	return relayconvert.ConverterOpenAIChatToClaudeMessagesStream
}

func (c *OpenAIToClaudeStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeStreamConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

// ConvertStreamResponse 读取 chat SSE reader，经 chunkWriter 输出 *dto.ClaudeStreamEvent。
func (c *OpenAIToClaudeStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	msgID := fmt.Sprintf("msg_%s", convmeta.RequestIDOf(info))
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
		if convmeta.ModelNameMappedOf(info) && info.HasChannelMeta() {
			modelName = info.GetUpstreamModelName()
		}
	}

	var (
		finishReason       string
		startSent          bool
		currentBlockType   string // "text" / "thinking" / "tool_use"
		contentIndex       int
		inputTokens        int
		outputTokens       int
		cachedTokens       int // OpenAI 口径：cached ⊆ prompt_tokens
		// toolIndexByUpstreamIndex：chat delta 的 tc.Index → 已开 tool_use 块的 contentIndex
		//（修复项：参数 delta 反查所属块；legacy 直接用当前 contentIndex 会错挂）
		toolBlockByUpstreamIndex = make(map[int]int)
	)

	emit := func(eventType string, data *dto.ClaudeResponse) error {
		return chunkWriter(&dto.ClaudeStreamEvent{Type: eventType, Data: data})
	}

	// closeCurrentBlock 关闭当前内容块（切块或收尾时调用）
	closeCurrentBlock := func() error {
		if currentBlockType == "" {
			return nil
		}
		if err := emit("content_block_stop", &dto.ClaudeResponse{
			Type:  "content_block_stop",
			Index: &contentIndex,
		}); err != nil {
			return err
		}
		contentIndex++
		currentBlockType = ""
		return nil
	}

	// emitMessageDelta 发送 message_delta（stop_reason + Claude 扣减口径 usage）+ message_stop
	emitMessageDeltaAndStop := func() error {
		reason := reasonmap.OpenAIFinishReasonToClaudeLegacySemantics(finishReason)
		deltaInput := inputTokens - cachedTokens
		if deltaInput < 0 {
			deltaInput = 0
		}
		if err := emit("message_delta", &dto.ClaudeResponse{
			Type: "message_delta",
			Delta: &dto.ClaudeDelta{
				StopReason: &reason,
			},
			Usage: &dto.ClaudeUsage{
				InputTokens:          deltaInput,
				CacheReadInputTokens: cachedTokens,
				OutputTokens:         outputTokens,
			},
		}); err != nil {
			return err
		}
		return emit("message_stop", &dto.ClaudeResponse{Type: "message_stop"})
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "event:") || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractClaudeSSEData(line)
		if data == "[DONE]" {
			if err := closeCurrentBlock(); err != nil {
				return err
			}
			return emitMessageDeltaAndStop()
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// usage 提取（任意 chunk，通常最后一个）
		if chunk.Usage != nil {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
			if chunk.Usage.PromptTokensDetails != nil {
				cachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
			}
		}

		// message_start（首个成功解析的 chunk 触发一次；此时 inputTokens 通常为 0，
		// 最终用量由 message_delta 补报）
		if !startSent {
			if chunk.Model != "" && !convmeta.ModelNameMappedOf(info) {
				modelName = chunk.Model
			}
			if err := emit("message_start", &dto.ClaudeResponse{
				Type: "message_start",
				Message: &dto.ClaudeMessageInfo{
					ID:      msgID,
					Type:    "message",
					Role:    "assistant",
					Content: []dto.ClaudeContentBlock{},
					Model:   modelName,
					Usage:   &dto.ClaudeUsage{InputTokens: inputTokens, OutputTokens: 0},
				},
			}); err != nil {
				return err
			}
			startSent = true
		}

		for _, choice := range chunk.Choices {
			// n>1 的多 choice 流在 Claude 单消息流无对应物，交错输出会损坏块流——只处理首个 choice
			if choice.Index > 0 {
				continue
			}
			// 文本 delta
			if text, ok := choice.Delta.Content.(string); ok && text != "" {
				if currentBlockType != "" && currentBlockType != "text" {
					if err := closeCurrentBlock(); err != nil {
						return err
					}
				}
				if currentBlockType != "text" {
					if err := emit("content_block_start", &dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: &contentIndex,
						ContentBlock: &dto.ClaudeContentBlock{
							Type: "text", Text: strPtrToLocal(""),
						},
					}); err != nil {
						return err
					}
					currentBlockType = "text"
				}
				if err := emit("content_block_delta", &dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: &contentIndex,
					Delta: &dto.ClaudeDelta{Type: "text_delta", Text: &text},
				}); err != nil {
					return err
				}
			}

			// 推理 delta（块切换/开启同 text 模式）
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				thinking := *choice.Delta.ReasoningContent
				if currentBlockType != "" && currentBlockType != "thinking" {
					if err := closeCurrentBlock(); err != nil {
						return err
					}
				}
				if currentBlockType != "thinking" {
					if err := emit("content_block_start", &dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: &contentIndex,
						ContentBlock: &dto.ClaudeContentBlock{
							Type: "thinking", Thinking: strPtrToLocal(""),
						},
					}); err != nil {
						return err
					}
					currentBlockType = "thinking"
				}
				if err := emit("content_block_delta", &dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: &contentIndex,
					Delta: &dto.ClaudeDelta{Type: "thinking_delta", Thinking: &thinking},
				}); err != nil {
					return err
				}
			}

			// 工具调用 delta
			for _, tc := range choice.Delta.ToolCalls {
				// 新调用判定：有 name（legacy 口径）
				if tc.Function.Name != "" {
					if err := closeCurrentBlock(); err != nil {
						return err
					}
					if err := emit("content_block_start", &dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: &contentIndex,
						ContentBlock: &dto.ClaudeContentBlock{
							Type: "tool_use", ID: tc.ID, Name: tc.Function.Name, Input: map[string]any{},
						},
					}); err != nil {
						return err
					}
					toolBlockByUpstreamIndex[tc.Index] = contentIndex
					currentBlockType = "tool_use"
				}

				// 参数 delta：按 tc.Index 反查所属块（修复项；未命中回退当前块——legacy 口径）
				if tc.Function.Arguments != "" {
					blockIndex := contentIndex
					if mapped, ok := toolBlockByUpstreamIndex[tc.Index]; ok {
						blockIndex = mapped
					}
					partial := tc.Function.Arguments
					if err := emit("content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: &blockIndex,
						Delta: &dto.ClaudeDelta{Type: "input_json_delta", PartialJSON: &partial},
					}); err != nil {
						return err
					}
				}
			}

			// finish_reason（只记录，收尾在 [DONE]/EOF 统一处理）
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 意外断流（无 [DONE] 的 EOF）：关块 + 补发 message_delta（修复项：legacy 只发
	// message_stop，客户端拿不到 stop_reason 与最终 usage）+ message_stop
	if err := closeCurrentBlock(); err != nil {
		return err
	}
	if !startSent {
		return nil // 空流：什么都不发（调用方按假成功防护处理）
	}
	return emitMessageDeltaAndStop()
}

// extractClaudeSSEData 从 SSE data 行提取数据部分。
func extractClaudeSSEData(line string) string {
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data)
}
