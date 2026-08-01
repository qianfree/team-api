package ollama_chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OllamaToOpenAIStreamConverter 将 Ollama Chat NDJSON 流转换为 OpenAI Chat Completions SSE。
//
// Ollama 流式为 NDJSON（每行一个 JSON 对象，无 SSE 前缀）：
// 最后一行 Done==true，携带 PromptEvalCount / EvalCount 用量。
type OllamaToOpenAIStreamConverter struct{}

func (c *OllamaToOpenAIStreamConverter) ID() string {
	return relayconvert.ResponseConverterOllamaChatToOAIChatStream
}

func (c *OllamaToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOllama
}

func (c *OllamaToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OllamaToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Ollama NDJSON 流转换为 OpenAI SSE 流。
func (c *OllamaToOpenAIStreamConverter) ConvertStreamResponse(
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

	newChunk := func(delta dto.Message, finishReason *string, usage *dto.UsageWithDetails) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "ollama"
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
	finishEmitted := false

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

		var ollamaResp dto.OllamaChatResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			continue
		}

		if ollamaResp.Done {
			usage := &dto.UsageWithDetails{
				PromptTokens:     ollamaResp.PromptEvalCount,
				CompletionTokens: ollamaResp.EvalCount,
				TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
			}
			reason := "stop"
			if err := chunkWriter(newChunk(dto.Message{}, &reason, usage)); err != nil {
				return err
			}
			finishEmitted = true
			return nil
		}

		// 内容增量（首个 chunk 带 role）
		delta := dto.Message{
			Content: ollamaResp.Message.Content,
		}
		if !roleChunkSent {
			delta.Role = "assistant"
			emptyContent := ""
			if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil, nil)); err != nil {
				return err
			}
			roleChunkSent = true
			delta.Role = ""
		}
		if err := chunkWriter(newChunk(delta, nil, nil)); err != nil {
			return err
		}
	}

	// 流异常结束（未收到 done）：补发终止 chunk
	if !finishEmitted {
		if !roleChunkSent {
			emptyContent := ""
			if err := chunkWriter(newChunk(dto.Message{Role: "assistant", Content: &emptyContent}, nil, nil)); err != nil {
				return err
			}
		}
		reason := "stop"
		if err := chunkWriter(newChunk(dto.Message{}, &reason, nil)); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}
	return nil
}
