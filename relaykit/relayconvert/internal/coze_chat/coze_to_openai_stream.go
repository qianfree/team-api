package coze_chat

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

// CozeToOpenAIStreamConverter 将 Coze SSE 流转换为 OpenAI Chat Completions SSE。
//
// Coze SSE 为事件类型化（event: + data: 行）：
//
//	event: conversation.message.delta
//	data: {"role":"assistant","type":"answer","content":"Hello"}
//
//	event: conversation.message.completed
//	data: {"role":"assistant","type":"answer","content":"Hello world"}
//
//	event: done
//	data: {}
type CozeToOpenAIStreamConverter struct{}

func (c *CozeToOpenAIStreamConverter) ID() string {
	return relayconvert.ResponseConverterCozeChatToOAIChatStream
}

func (c *CozeToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatCoze
}

func (c *CozeToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *CozeToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityFair
}

// ConvertStreamResponse 将 Coze SSE 流转换为 OpenAI SSE 流。
func (c *CozeToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	responseID := fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	newChunk := func(delta dto.Message, finishReason *string) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "coze"
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
		return chunk
	}

	var (
		currentEvent string
		roleChunkSent bool
		finishEmitted bool
	)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()

		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}

		switch currentEvent {
		case "conversation.message.delta":
			var msg dto.CozeMessage
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue
			}
			// 只转发 answer 类型
			if msg.Type != "answer" || msg.Content == "" {
				continue
			}
			if !roleChunkSent {
				emptyContent := ""
				if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil)); err != nil {
					return err
				}
				roleChunkSent = true
			}
			if err := chunkWriter(newChunk(dto.Message{Content: msg.Content}, nil)); err != nil {
				return err
			}

		case "conversation.message.completed":
			// 完成事件：发送带 finish_reason 的最终 chunk
			reason := "stop"
			if err := chunkWriter(newChunk(dto.Message{}, &reason)); err != nil {
				return err
			}
			finishEmitted = true

		case "done":
			// 流结束
			if !finishEmitted {
				reason := "stop"
				if err := chunkWriter(newChunk(dto.Message{}, &reason)); err != nil {
					return err
				}
			}
			return nil

		case "error":
			return fmt.Errorf("coze stream error: %s", data)
		}
	}

	// 流异常结束（未收到 done/completed）：补发终止 chunk
	if !finishEmitted {
		if !roleChunkSent {
			emptyContent := ""
			if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil)); err != nil {
				return err
			}
		}
		reason := "stop"
		if err := chunkWriter(newChunk(dto.Message{}, &reason)); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}
	return nil
}
