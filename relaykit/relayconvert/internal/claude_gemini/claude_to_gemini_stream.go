package claude_gemini

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

// geminiToolCallState 流式聚合中的工具调用（Claude tool_use 块）。
// Claude 的参数按 input_json_delta 分片下发，而 Gemini 的 functionCall.args 必须是
// 完整对象，因此必须缓冲到 content_block_stop 才能发出。
type geminiToolCallState struct {
	id   string
	name string
	args strings.Builder
}

// ClaudeToGeminiStreamConverter 将 Claude SSE 流转换为 Gemini SSE 流。
// 事件形态与 Gemini 出站保持一致：逐 chunk 输出纯 data 帧（StreamEvent.Event 为空），
// 末尾补 finishReason + usageMetadata 的收尾 chunk（同时携带计费用量）；
// [DONE] 帧由宿主桥接层写出。
type ClaudeToGeminiStreamConverter struct{}

func (c *ClaudeToGeminiStreamConverter) ID() string {
	return relayconvert.ResponseConverterClaudeMessagesToGeminiChatStream
}

func (c *ClaudeToGeminiStreamConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToGeminiStreamConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *ClaudeToGeminiStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Claude SSE 事件流转换为 Gemini data 帧序列。
// reader 提供上游 Claude 的原生 SSE；每个「写事件」以 chunkWriter(StreamEvent) 交宿主帧化。
func (c *ClaudeToGeminiStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	modelName := originModelName(info)

	var (
		usage       dto.ClaudeUsage
		stopReason  string
		currentTool *geminiToolCallState
	)

	// emitParts 发出一个仅含 content parts 的 Gemini chunk
	emitParts := func(parts []dto.GeminiPart) error {
		if len(parts) == 0 {
			return nil
		}
		chunk := &dto.GeminiChatResponse{
			ModelName: modelName,
			Candidates: []dto.GeminiCandidate{{
				Content: &dto.GeminiContent{Role: "model", Parts: parts},
			}},
		}
		return chunkWriter(&relayconvert.StreamEvent{Data: chunk})
	}

	// flushTool 把缓冲的工具调用作为 functionCall part 发出
	flushTool := func() error {
		if currentTool == nil {
			return nil
		}
		var args any
		if raw := currentTool.args.String(); raw != "" {
			if err := json.Unmarshal([]byte(raw), &args); err != nil {
				// 参数分片拼接后仍非法：降级为空对象，保留函数名让客户端可见调用意图
				args = map[string]any{}
			}
		} else {
			args = map[string]any{}
		}
		err := emitParts([]dto.GeminiPart{{
			FunctionCall: &dto.GeminiFunctionCall{
				// Gemini 的 functionCall 支持可选 id，直接搬运 Claude 的 tool_use.id
				ID:           currentTool.id,
				FunctionName: currentTool.name,
				Arguments:    args,
			},
		}})
		currentTool = nil
		return err
	}

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			// 流中断：计费兜底（输出估算、输入补齐）由宿主桥接层处理
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

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// JSON 解析失败：静默跳过（允许部分格式异常）
			continue
		}

		switch event.Type {
		case "message_start":
			if event.Message != nil {
				// 模型映射时不回填上游模型名，避免泄漏给客户端
				if event.Message.Model != "" && !isModelMapped(info) {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			if event.ContentBlock.Type == "tool_use" {
				// 上一个工具块理论上已在 content_block_stop 冲刷，这里兜底防止串块
				if err := flushTool(); err != nil {
					return err
				}
				currentTool = &geminiToolCallState{
					id:   event.ContentBlock.ID,
					name: event.ContentBlock.Name,
				}
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					if err := emitParts([]dto.GeminiPart{{Text: *event.Delta.Text}}); err != nil {
						return err
					}
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					if err := emitParts([]dto.GeminiPart{{Text: *event.Delta.Thinking, Thought: boolPtr(true)}}); err != nil {
						return err
					}
				}
			case "signature_delta":
				if event.Delta.Signature != "" {
					// 思考签名单独成 part（无文本），与 Gemini 把签名挂在 thought part 上的形态一致
					if err := emitParts([]dto.GeminiPart{{Thought: boolPtr(true), ThoughtSignature: event.Delta.Signature}}); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if currentTool != nil && event.Delta.PartialJSON != nil {
					currentTool.args.WriteString(*event.Delta.PartialJSON)
				}
			}

		case "content_block_stop":
			if err := flushTool(); err != nil {
				return err
			}

		case "message_delta":
			if event.Delta != nil && event.Delta.StopReason != nil {
				stopReason = *event.Delta.StopReason
			}
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

		case "error":
			// 与旧桥一致：仅记录、不中断——收尾 chunk 仍会发出，避免客户端拿不到 finishReason。
			// 错误上报（StreamStatus）属宿主职责，本层不承载。

		case "message_stop":
			// 收尾在循环外统一处理，保证异常结束路径也走同一套
		}
	}

	// 未闭合的工具块（上游异常断流）：兜底冲刷，避免整个调用丢失
	if err := flushTool(); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil && err != io.EOF && ctx.Err() == nil {
		// 读流出错：已累计的 usage 无法随错误返回（StreamEvent 只随帧携带），由宿主按估算兜底
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 收尾 chunk：finishReason + usageMetadata（与 Gemini 出站的既有形态一致），
	// 同时携带 Claude 计费口径的用量供宿主捕获（input 不含缓存、cache_creation 独立计价）
	finalChunk := &dto.GeminiChatResponse{
		ModelName: modelName,
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model"},
			FinishReason: claudeStopReasonToGemini(stopReason),
		}},
	}
	if um := geminiUsageFromClaude(&usage); um != nil {
		finalChunk.UsageMetadata = um
	}
	return chunkWriter(&relayconvert.StreamEvent{
		Data:  finalChunk,
		Usage: claudeBillingUsage(&usage),
	})
}
