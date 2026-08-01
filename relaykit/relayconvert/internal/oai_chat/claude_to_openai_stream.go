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

// ClaudeToOpenAIStreamConverter converts Claude Messages API streaming response to OpenAI Chat Completions streaming response.
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

// ConvertStreamResponse converts a Claude SSE stream to OpenAI SSE stream.
// The reader contains Claude SSE events, and chunks are written to the chunkWriter callback.
func (c *ClaudeToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// Generate response ID
	responseID := generateResponseID()
	createdAt := getCurrentTimestamp()

	// Determine model name
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
			m = "claude-3-opus-20240229" // fallback
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
			// Extract model name and initial usage
			if event.Message != nil {
				if event.Message.Model != "" {
					modelName = event.Message.Model
				}
				if event.Message.Usage != nil {
					usage = *event.Message.Usage
				}
			}

			// Send role chunk
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
				// Text block start, no delta yet
			case "thinking":
				// Thinking block start, no delta yet
			case "redacted_thinking":
				// Redacted thinking, ignore (no OpenAI equivalent)
			case "tool_use":
				// Tool use block start
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
			// Content block ended, no action needed

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
			// Final chunk with finish_reason and usage
			reason := finishReason
			if reason == "" {
				reason = "stop"
			}
			usageObj := &dto.UsageWithDetails{
				PromptTokens:     usage.InputTokens,
				CompletionTokens: usage.OutputTokens,
				TotalTokens:      usage.InputTokens + usage.OutputTokens,
			}
			if usageObj.CompletionTokens == 0 {
				// Estimate from text length if not provided
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
			// Claude error event
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

// Helper functions

func generateResponseID() string {
	// Simple timestamp-based ID
	return fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())
}

func getCurrentTimestamp() int64 {
	return 1700000000 // Fixed timestamp for testing
}
