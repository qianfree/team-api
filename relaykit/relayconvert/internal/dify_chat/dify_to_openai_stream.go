package dify_chat

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

// DifyToOpenAIStreamConverter 将 Dify streaming SSE 转换为 OpenAI Chat Completions SSE。
type DifyToOpenAIStreamConverter struct{}

func (c *DifyToOpenAIStreamConverter) ID() string {
	return relayconvert.ResponseConverterDifyChatToOAIChatStream
}

func (c *DifyToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatDify
}

func (c *DifyToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *DifyToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityFair
}

// ConvertStreamResponse 将 Dify SSE 流转换为 OpenAI SSE 流。
// Dify SSE 帧形如：data: {"event":"message","answer":"chunk"} / {"event":"message_end",...}
func (c *DifyToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	responseID := fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	newChunk := func(delta dto.Message, finishReason *string, usage *dto.UsageWithDetails) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "dify"
		}
		chunk := &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: 0,
			Model:   m,
			Choices: []dto.StreamChoice{{
				Index: 0,
				Delta: delta,
			}},
		}
		if finishReason != nil {
			chunk.Choices[0].FinishReason = finishReason
		}
		if usage != nil {
			chunk.Usage = usage
		}
		return chunk
	}

	roleChunkSent := false
	var capturedUsage dto.UsageWithDetails

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var event dto.DifyStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Event {
		case "message":
			if event.Answer == "" {
				continue
			}
			// 首个内容 chunk 前补一个 role chunk
			if !roleChunkSent {
				emptyContent := ""
				if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil, nil)); err != nil {
					return err
				}
				roleChunkSent = true
			}
			if err := chunkWriter(newChunk(dto.Message{Content: event.Answer}, nil, nil)); err != nil {
				return err
			}

		case "message_end":
			capturedUsage = dto.UsageWithDetails{
				PromptTokens:     event.Metadata.Usage.PromptTokens,
				CompletionTokens: event.Metadata.Usage.CompletionTokens,
				TotalTokens:      event.Metadata.Usage.TotalTokens,
			}
			reason := "stop"
			var usagePtr *dto.UsageWithDetails
			if capturedUsage.TotalTokens > 0 || capturedUsage.PromptTokens > 0 || capturedUsage.CompletionTokens > 0 {
				u := capturedUsage
				usagePtr = &u
			}
			if err := chunkWriter(newChunk(dto.Message{}, &reason, usagePtr)); err != nil {
				return err
			}
			return nil

		case "error":
			return fmt.Errorf("dify stream error: %s", data)
		}
	}

	// 未收到 message_end（流异常结束）：补发一个终止 chunk
	if !roleChunkSent {
		// 全程无内容，先补 role chunk
		emptyContent := ""
		if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil, nil)); err != nil {
			return err
		}
	}
	reason := "stop"
	var usagePtr *dto.UsageWithDetails
	if capturedUsage.TotalTokens > 0 || capturedUsage.PromptTokens > 0 || capturedUsage.CompletionTokens > 0 {
		u := capturedUsage
		usagePtr = &u
	}
	if err := chunkWriter(newChunk(dto.Message{}, &reason, usagePtr)); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}
	return nil
}
