package oai_gemini

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
	"github.com/qianfree/team-api/relaykit/relayconvert/internal/shared"
	"github.com/qianfree/team-api/relaykit/types"
)

// GeminiToOpenAIStreamConverter 将 Gemini 流式响应转换为 OpenAI Chat Completions 流式响应。
type GeminiToOpenAIStreamConverter struct{}

func (c *GeminiToOpenAIStreamConverter) ID() string {
	return relayconvert.ResponseConverterGeminiChatToOAIChatStream
}

func (c *GeminiToOpenAIStreamConverter) From() types.RelayFormat {
	return types.RelayFormatGemini
}

func (c *GeminiToOpenAIStreamConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *GeminiToOpenAIStreamConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityGood
}

// ConvertStreamResponse 将 Gemini SSE 流转换为 OpenAI SSE 流。
func (c *GeminiToOpenAIStreamConverter) ConvertStreamResponse(
	ctx context.Context,
	info convmeta.Meta,
	reader io.Reader,
	chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	// 生成响应 ID
	responseID := fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp())

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	var (
		totalUsage    dto.GeminiUsageMetadata
		finishReason  string
		toolCallIdx   int
		roleChunkSent bool
		citations     *shared.GroundingCitations
		answerText    strings.Builder
	)

	newChunk := func(delta dto.Message) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "gemini-pro"
		}
		return &dto.ChatCompletionStreamResponse{
			ID:      responseID,
			Object:  "chat.completion.chunk",
			Created: 0,
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

		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
			continue
		}

		// 收集 usage
		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}

		// 检查 prompt feedback 中的安全拦截
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			return fmt.Errorf("request blocked by Gemini safety filter: %s: %w", geminiResp.PromptFeedback.BlockReason, relayconvert.ErrContentBlocked)
		}

		// 收集模型名
		if geminiResp.ModelName != "" {
			modelName = geminiResp.ModelName
		}

		// 若尚未发送 role chunk 则发送
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

		for _, candidate := range geminiResp.Candidates {
			if candidate.FinishReason != "" {
				finishReason = mapGeminiFinishReason(candidate.FinishReason)
			}
			// grounding 随末帧（或靠后的帧）到达，先捕获、末帧统一附带引用
			if c := shared.ParseGeminiGrounding(candidate.GroundingMetadata); !c.IsEmpty() {
				citations = c
			}

			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				isThought := part.Thought != nil && *part.Thought

				// 文本内容
				if part.Text != "" {
					if isThought {
						// thinking 内容 → reasoning_content
						if err := chunkWriter(newChunk(dto.Message{
							ReasoningContent: &part.Text,
						})); err != nil {
							return err
						}
					} else {
						answerText.WriteString(part.Text)
						if err := chunkWriter(newChunk(dto.Message{
							Content: part.Text,
						})); err != nil {
							return err
						}
					}
				}

				// 内联图片数据
				if part.InlineData != nil {
					imageMarkdown := fmt.Sprintf("![image](data:%s;base64,%s)", part.InlineData.MimeType, part.InlineData.Data)
					if err := chunkWriter(newChunk(dto.Message{
						Content: imageMarkdown,
					})); err != nil {
						return err
					}
				}

				// 文件数据
				if part.FileData != nil {
					fileMarkdown := fmt.Sprintf("[file](%s)", part.FileData.FileURI)
					if err := chunkWriter(newChunk(dto.Message{
						Content: fileMarkdown,
					})); err != nil {
						return err
					}
				}

				// 可执行代码
				if part.ExecutableCode != nil {
					codeBlock := fmt.Sprintf("```%s\n%s\n```", part.ExecutableCode.Language, part.ExecutableCode.Code)
					if err := chunkWriter(newChunk(dto.Message{
						Content: codeBlock,
					})); err != nil {
						return err
					}
				}

				// 代码执行结果
				if part.CodeExecutionResult != nil {
					resultText := fmt.Sprintf("Execution %s:\n%s", part.CodeExecutionResult.Outcome, part.CodeExecutionResult.Output)
					if err := chunkWriter(newChunk(dto.Message{
						Content: resultText,
					})); err != nil {
						return err
					}
				}

				// 函数调用
				if part.FunctionCall != nil {
					argsJSON, _ := json.Marshal(part.FunctionCall.Arguments)
					// Gemini 每个函数调用整体到达，但 OpenAI 流式契约要求每个
					// tool_call 增量都带 index（0 也要发），客户端按 index 归并分片
					idx := toolCallIdx
					if err := chunkWriter(newChunk(dto.Message{
						ToolCalls: []dto.ToolCall{{
							Index: &idx,
							ID:    fmt.Sprintf("call_%s_%d", responseID, toolCallIdx),
							Type:  "function",
							Function: dto.FunctionCall{
								Name:      part.FunctionCall.FunctionName,
								Arguments: string(argsJSON),
							},
						}},
					})); err != nil {
						return err
					}
					toolCallIdx++
				}
			}
		}
	}

	// 发送带 finish_reason 和 usage 的最终 chunk
	reason := finishReason
	if reason == "" {
		reason = "stop"
	}

	finalChunk := &dto.ChatCompletionStreamResponse{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   modelName,
		Choices: []dto.StreamChoice{{
			Index:        0,
			FinishReason: &reason,
		}},
	}

	// 服务端搜索证据 → 末帧 delta 的 annotations。
	// 只能挂在末帧：groundingMetadata 随末帧到达，此时正文增量早已推给客户端；
	// 要按来源逐段插入就得缓冲整段正文，那会毁掉流式首字延迟。
	if !citations.IsEmpty() {
		citations.AlignSupports(answerText.String())
		finalChunk.Choices[0].Delta.Annotations = citations.ToOpenAIAnnotations()
	}

	if totalUsage.PromptTokenCount > 0 || totalUsage.CandidatesTokenCount > 0 {
		// OpenAI 口径：prompt 含 cached（子集），completion 含 thoughts（子集）——
		// Gemini 的 candidatesTokenCount 不含思考 token，须与 thoughtsTokenCount 合计，
		// 否则计费捕获与客户端按 OpenAI 语义解析都会漏掉思考部分
		finalChunk.Usage = &dto.UsageWithDetails{
			PromptTokens:     totalUsage.PromptTokenCount,
			CompletionTokens: totalUsage.CandidatesTokenCount + totalUsage.ThoughtsTokenCount,
			TotalTokens:      totalUsage.TotalTokenCount,
		}
		if totalUsage.CachedContentTokenCount > 0 {
			finalChunk.Usage.PromptTokensDetails = &dto.TokenDetails{CachedTokens: totalUsage.CachedContentTokenCount}
		}
		if totalUsage.ThoughtsTokenCount > 0 {
			finalChunk.Usage.CompletionTokenDetails = &dto.TokenDetails{ReasoningTokens: totalUsage.ThoughtsTokenCount}
		}
	}

	if err := chunkWriter(finalChunk); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	return nil
}
