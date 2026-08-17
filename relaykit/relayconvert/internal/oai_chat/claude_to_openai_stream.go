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

// ClaudeToOpenAIStreamConverter 将 Claude Messages API 流式响应转换为 OpenAI Chat Completions 流式响应。
type ClaudeToOpenAIStreamConverter struct{}

func (c *ClaudeToOpenAIStreamConverter) ID() string {
	return relayconvert.ConverterClaudeMessagesToOpenAIChatStream
}

func (c *ClaudeToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatClaude
}

func (c *ClaudeToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *ClaudeToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Claude SSE 流转换为 OpenAI SSE 流。
// reader 提供 Claude 的 SSE 事件，转换后的 chunk 通过 chunkWriter 回调写出。
func (c *ClaudeToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// 生成响应 ID
	responseID := generateResponseID()
	createdAt := getCurrentTimestamp()

	// 确定模型名
	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	var (
		usage           dto.ClaudeUsage
		finishReason    string
		toolCallIdx     int
		roleChunkSent   bool
		responseTextBuf strings.Builder
	)

	newChunk := func(delta dto.Message) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "claude-3-opus-20240229" // 兜底值
		}
		return &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: createdAt,
			Model:   m,
			Choices: []dto.StreamChoice{{
				Index: 0,
				Delta: delta,
			}},
		}
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

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.ClaudeResponse
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			// 提取模型名与初始 usage
			if event.Message != nil {
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}

			// 发送 role chunk
			if !roleChunkSent {
				emptyContent := ""
				if err := chunkWriter(newChunk(dto.Message{
					Role:    "assistant",
					Content: &emptyContent,
				})); err != nil {
					return err
				}
				roleChunkSent = true
			}

		case "content_block_start":
			if event.ContentBlock == nil {
				continue
			}
			switch event.ContentBlock.Type {
			case "text":
				// text 块开始，尚无 delta
			case "thinking":
				// thinking 块开始，尚无 delta
			case "redacted_thinking":
				// 已脱敏的 thinking，忽略（OpenAI 无对应概念）
			case "tool_use":
				// tool_use 块开始
				toolCall := dto.ToolCall{
					Index: toolCallIdx,
					ID:    event.ContentBlock.ID,
					Type:  "function",
					Function: dto.FunctionCall{
						Name:      event.ContentBlock.Name,
						Arguments: "",
					},
				}
				if err := chunkWriter(newChunk(dto.Message{
					ToolCalls: []dto.ToolCall{toolCall},
				})); err != nil {
					return err
				}
				toolCallIdx++
			}

		case "content_block_delta":
			if event.Delta == nil {
				continue
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != nil && *event.Delta.Text != "" {
					responseTextBuf.WriteString(*event.Delta.Text)
					if err := chunkWriter(newChunk(dto.Message{
						Content: *event.Delta.Text,
					})); err != nil {
						return err
					}
				}
			case "thinking_delta":
				if event.Delta.Thinking != nil && *event.Delta.Thinking != "" {
					if err := chunkWriter(newChunk(dto.Message{
						ReasoningContent: event.Delta.Thinking,
					})); err != nil {
						return err
					}
				}
			case "input_json_delta":
				if event.Delta.PartialJSON != nil && *event.Delta.PartialJSON != "" {
					if err := chunkWriter(newChunk(dto.Message{
						ToolCalls: []dto.ToolCall{{
							Index: toolCallIdx - 1,
							Function: dto.FunctionCall{
								Arguments: *event.Delta.PartialJSON,
							},
						}},
					})); err != nil {
						return err
					}
				}
			}

		case "content_block_stop":
			// content block 结束，无需处理

		case "message_delta":
			if event.Delta != nil {
				if event.Delta.StopReason != nil {
					finishReason = mapClaudeStopReasonToOpenAI(*event.Delta.StopReason)
				}
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
			}

		case "message_stop":
			// 末尾 chunk，包含 finish_reason 和 usage。
			// Claude 的 input_tokens 不含缓存（三项并列），OpenAI 的 prompt_tokens 含缓存
			//（cached 是其子集），转换必须做加法，否则缓存场景下客户端少算输入量
			reason := finishReason
			if reason == "" {
				reason = "stop"
			}
			promptTotal := usage.InputTokens + usage.CacheReadInputTokens + usage.CacheCreationInputTokens
			usageObj := &dto.UsageWithDetails{
				PromptTokens:     promptTotal,
				CompletionTokens: usage.OutputTokens,
				TotalTokens:      promptTotal + usage.OutputTokens,
			}
			if usage.CacheReadInputTokens > 0 || usage.CacheCreationInputTokens > 0 {
				usageObj.PromptTokensDetails = &dto.TokenDetails{
					CachedTokens:         usage.CacheReadInputTokens,
					CachedCreationTokens: usage.CacheCreationInputTokens,
				}
			}
			if usageObj.CompletionTokens == 0 {
				// 未提供时根据文本长度估算
				estimated := responseTextBuf.Len() / 4
				if estimated > 0 {
					usageObj.CompletionTokens = estimated
					usageObj.TotalTokens = usageObj.PromptTokens + usageObj.CompletionTokens
				}
			}

			chunk := &dto.ChatCompletionStreamResponse{
				ID:      responseID,
				Object:  "chat.completion.chunk",
				Created: createdAt,
				Model:   modelName,
				Choices: []dto.StreamChoice{{
					Index:        0,
					FinishReason: &reason,
				}},
				Usage: usageObj,
			}
			if chunk.Model == "" && info != nil {
				chunk.Model = info.GetOriginModelName()
			}
			if err := chunkWriter(chunk); err != nil {
				return err
			}

		case "error":
			// Claude error 事件
			errMsg := "claude stream error"
			if event.Error != nil {
				if b, err := json.Marshal(event.Error); err == nil {
					errMsg = string(b)
				}
			}
			return fmt.Errorf("claude stream error: %s", errMsg)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	return nil
}

// 辅助函数

func generateResponseID() string {
	// 基于时间戳的简单 ID
	return fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())
}

func getCurrentTimestamp() int64 {
	return 1700000000 // 测试用的固定时间戳
}
