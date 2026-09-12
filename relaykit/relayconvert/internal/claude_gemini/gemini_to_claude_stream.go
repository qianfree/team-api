package claude_gemini

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

// GeminiToClaudeStreamConverter 将 Gemini SSE 流转换为 Claude Messages 事件流。
// 事件序列与 Claude 出站保持一致（message_start → content_block_* → message_delta → message_stop），
// 每个事件以 StreamEvent{Event: 事件名} 输出，由宿主桥接层写为事件帧。
type GeminiToClaudeStreamConverter struct{}

func (c *GeminiToClaudeStreamConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToClaudeMessagesStream
}

func (c *GeminiToClaudeStreamConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToClaudeStreamConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *GeminiToClaudeStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Gemini SSE 流转换为 Claude 事件帧序列。
// reader 提供上游 Gemini 的原生 SSE（Code Assist 包装的解包在宿主侧完成）。
func (c *GeminiToClaudeStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// 消息/工具 ID 基准：宿主 RequestID 未随 Meta 下沉，以纳秒时间戳保证单响应内唯一
	idBase := fmt.Sprintf("%d", time.Now().UnixNano())
	msgID := fmt.Sprintf("msg_%s", idBase)
	modelName := originModelName(info)

	var (
		totalUsage       dto.GeminiUsageMetadata
		startSent        bool
		finishReason     string
		hasToolUse       bool
		contentIndex     int
		currentBlockType string
		toolIdx          int
		citations        *shared.GroundingCitations
		answerText       strings.Builder
	)

	// emitEvent 发出一个 Claude 格式事件帧
	emitEvent := func(eventType string, data *dto.ClaudeResponse, usage *dto.UsageWithDetails) error {
		return chunkWriter(&relayconvert.StreamEvent{Event: eventType, Data: data, Usage: usage})
	}

	// closeBlock 关闭当前 content block 并推进索引
	closeBlock := func() error {
		if currentBlockType == "" {
			return nil
		}
		if err := emitEvent("content_block_stop", &dto.ClaudeResponse{
			Type:  "content_block_stop",
			Index: intPtr(contentIndex),
		}, nil); err != nil {
			return err
		}
		contentIndex++
		currentBlockType = ""
		return nil
	}

	// openTextLikeBlock 开启 text/thinking 块；同类型连续增量复用当前块
	openTextLikeBlock := func(blockType string, block *dto.ClaudeContentBlock) error {
		if currentBlockType == blockType {
			return nil
		}
		if err := closeBlock(); err != nil {
			return err
		}
		if err := emitEvent("content_block_start", &dto.ClaudeResponse{
			Type:         "content_block_start",
			Index:        intPtr(contentIndex),
			ContentBlock: block,
		}, nil); err != nil {
			return err
		}
		currentBlockType = blockType
		return nil
	}

	// emitSearchBlocks 在流末尾补发服务端搜索证据块（server_tool_use + web_search_tool_result）。
	//
	// 位置只能在末尾：Gemini 的 groundingMetadata 随末帧（或靠后的帧）到达，而此时正文
	// 增量早已推给客户端。要还原成 Claude 那样「搜索在前、回答在后」的顺序就得缓冲整段正文，
	// 那会毁掉流式的首字延迟——宁可顺序不同，也不牺牲流式体验；证据本身不丢即达成守恒。
	emitSearchBlocks := func() error {
		if citations.IsEmpty() {
			return nil
		}
		citations.AlignSupports(answerText.String())
		blocks := citations.ToClaudeSearchBlocks(idBase)
		for i := range blocks {
			block := blocks[i]
			// server_tool_use 的 input 按协议经 input_json_delta 增量下发：
			// 只放在 content_block_start 里的话，只累加增量的 SDK 会拿到空 input
			var inputJSON string
			if block.Type == "server_tool_use" && block.Input != nil {
				raw, err := json.Marshal(block.Input)
				if err != nil {
					return err
				}
				inputJSON = string(raw)
				block.Input = map[string]any{}
			}

			if err := emitEvent("content_block_start", &dto.ClaudeResponse{
				Type:         "content_block_start",
				Index:        intPtr(contentIndex),
				ContentBlock: &block,
			}, nil); err != nil {
				return err
			}
			if inputJSON != "" {
				if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
					Type:  "content_block_delta",
					Index: intPtr(contentIndex),
					Delta: &dto.ClaudeDelta{Type: "input_json_delta", PartialJSON: strPtr(inputJSON)},
				}, nil); err != nil {
					return err
				}
			}
			if err := emitEvent("content_block_stop", &dto.ClaudeResponse{
				Type:  "content_block_stop",
				Index: intPtr(contentIndex),
			}, nil); err != nil {
				return err
			}
			contentIndex++
		}
		return nil
	}

	// emitFinal 发送 message_delta（stop_reason + 最终用量）与 message_stop。
	// attachUsage 控制是否随 message_delta 帧携带计费用量（安全过滤路径与旧桥一致不携带）。
	emitFinal := func(attachUsage bool) error {
		if err := closeBlock(); err != nil {
			return err
		}
		if err := emitSearchBlocks(); err != nil {
			return err
		}

		stopReason := geminiFinishReasonToClaude(finishReason)
		if hasToolUse {
			// Gemini 带 functionCall 时 finishReason 仍是 STOP，Claude 协议要求 tool_use，
			// 否则客户端不会进入工具调用回合
			stopReason = claudeToolUse
		}

		var billing *dto.UsageWithDetails
		if attachUsage {
			billing = geminiBillingUsage(&totalUsage)
		}
		if err := emitEvent("message_delta", &dto.ClaudeResponse{
			Type:  "message_delta",
			Delta: &dto.ClaudeDelta{StopReason: strPtr(stopReason)},
			Usage: claudeUsageFromGemini(&totalUsage),
		}, billing); err != nil {
			return err
		}
		return emitEvent("message_stop", &dto.ClaudeResponse{Type: "message_stop"}, nil)
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
		if geminiResp.ModelName != "" {
			modelName = geminiResp.ModelName
		}

		// 检查 promptFeedback.blockReason（流式安全过滤）
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			// 事件流已开场，必须补齐 Claude 的收尾事件，否则客户端挂起等 message_stop
			finishReason = "SAFETY"
			if err := emitFinal(false); err != nil {
				return err
			}
			return fmt.Errorf("request blocked by Gemini safety filter: %s: %w", geminiResp.PromptFeedback.BlockReason, relayconvert.ErrContentBlocked)
		}

		// message_start：首个有效 chunk 时发送。Gemini 的 usageMetadata 逐块累计，
		// 首块的 promptTokenCount 已可用，直接带上
		if !startSent {
			if err := emitEvent("message_start", &dto.ClaudeResponse{
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
						InputTokens:  claudeInputTokens(&totalUsage),
						OutputTokens: 0,
					},
				},
			}, nil); err != nil {
				return err
			}
			startSent = true
		}

		for _, candidate := range geminiResp.Candidates {
			if candidate.FinishReason != "" {
				finishReason = candidate.FinishReason
			}
			// grounding 随末帧（或靠后的帧）到达，先捕获、流末尾统一补发证据块
			if c := shared.ParseGeminiGrounding(candidate.GroundingMetadata); !c.IsEmpty() {
				citations = c
			}
			if candidate.Content == nil {
				continue
			}

			for i := range candidate.Content.Parts {
				part := &candidate.Content.Parts[i]
				isThought := part.Thought != nil && *part.Thought

				// 文本 / 思考增量
				if part.Text != "" {
					if isThought {
						if err := openTextLikeBlock("thinking", &dto.ClaudeContentBlock{
							Type:     "thinking",
							Thinking: strPtr(""),
						}); err != nil {
							return err
						}
						if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
							Type:  "content_block_delta",
							Index: intPtr(contentIndex),
							Delta: &dto.ClaudeDelta{Type: "thinking_delta", Thinking: strPtr(part.Text)},
						}, nil); err != nil {
							return err
						}
					} else {
						if err := openTextLikeBlock("text", &dto.ClaudeContentBlock{
							Type: "text",
							Text: strPtr(""),
						}); err != nil {
							return err
						}
						answerText.WriteString(part.Text)
						if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
							Type:  "content_block_delta",
							Index: intPtr(contentIndex),
							Delta: &dto.ClaudeDelta{Type: "text_delta", Text: strPtr(part.Text)},
						}, nil); err != nil {
							return err
						}
					}
				}

				// thoughtSignature 与 Claude 的 thinking.signature 同为对客户端不透明的载体，
				// 原样搬运；仅在 thinking 块内有意义
				if part.ThoughtSignature != "" && currentBlockType == "thinking" {
					if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "signature_delta", Signature: part.ThoughtSignature},
					}, nil); err != nil {
						return err
					}
				}

				// 工具调用：Gemini 一次给全量参数，不做增量拼接
				if part.FunctionCall != nil {
					if err := closeBlock(); err != nil {
						return err
					}
					if err := emitEvent("content_block_start", &dto.ClaudeResponse{
						Type:  "content_block_start",
						Index: intPtr(contentIndex),
						ContentBlock: &dto.ClaudeContentBlock{
							Type:  "tool_use",
							ID:    claudeToolUseID(idBase, toolIdx),
							Name:  part.FunctionCall.FunctionName,
							Input: map[string]any{},
						},
					}, nil); err != nil {
						return err
					}
					currentBlockType = "tool_use"
					toolIdx++
					hasToolUse = true

					argsJSON, err := json.Marshal(geminiFunctionArgs(part.FunctionCall))
					if err != nil {
						argsJSON = []byte("{}")
					}
					if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "input_json_delta", PartialJSON: strPtr(string(argsJSON))},
					}, nil); err != nil {
						return err
					}
					if err := closeBlock(); err != nil {
						return err
					}
				}

				// Claude assistant 内容块无对应类型，按既有 Gemini→OpenAI 口径渲染为文本
				if fallback := geminiPartFallbackText(part); fallback != "" {
					if err := openTextLikeBlock("text", &dto.ClaudeContentBlock{Type: "text", Text: strPtr("")}); err != nil {
						return err
					}
					if err := emitEvent("content_block_delta", &dto.ClaudeResponse{
						Type:  "content_block_delta",
						Index: intPtr(contentIndex),
						Delta: &dto.ClaudeDelta{Type: "text_delta", Text: strPtr(fallback)},
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

	// 上游一个有效 chunk 都没给：仍需补齐 message_start，否则客户端拿不到完整事件序列
	if !startSent {
		if err := emitEvent("message_start", &dto.ClaudeResponse{
			Type: "message_start",
			Message: &dto.ClaudeMessageInfo{
				ID:      msgID,
				Type:    "message",
				Role:    "assistant",
				Content: []dto.ClaudeContentBlock{},
				Model:   modelName,
				Usage:   &dto.ClaudeUsage{},
			},
		}, nil); err != nil {
			return err
		}
	}

	return emitFinal(true)
}
