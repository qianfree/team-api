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

	// ID：请求 ID 合成（缺失时用 NowFunc 兜底）；时间戳取转换开始时刻
	requestID := convmeta.RequestIDOf(info)
	if requestID == "" {
		requestID = fmt.Sprintf("%d", NowFunc().UnixNano())
	}
	responseID := fmt.Sprintf("chatcmpl-%s", requestID)
	createdAt := NowFunc().Unix()

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	var (
		totalUsage       dto.GeminiUsageMetadata
		finishReason     string
		toolCallIdx      int
		roleChunkSent    bool
		parsedChunks     int  // 成功解析的 data 行数（假成功防护的诊断信息）
		sawGeminiPayload bool // 是否出现过 candidates / usageMetadata —— Gemini 流的协议特征
	)

	// mismatchIfEmpty 假成功防护：整段上游流没有任何目标协议特征时报 ErrProtocolMismatch，
	// 由宿主桥接层按上游错误处理并置 StreamEndReasonError。绝不能静默收尾成空响应——
	// 客户端只会收到补发的终止事件而无任何内容，且该次请求在健康度上被记为成功、
	// 调度 FSM 失去换渠道机会。
	mismatchIfEmpty := func() error {
		if sawGeminiPayload {
			return nil
		}
		return fmt.Errorf("%w: %d chunks parsed, none contained candidates", types.ErrProtocolMismatch, parsedChunks)
	}

	newChunk := func(delta dto.Message) *dto.ChatCompletionStreamResponse {
		m := modelName
		if m == "" {
			m = "gemini-pro"
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

		var geminiResp dto.GeminiChatResponse
		if err := json.Unmarshal([]byte(data), &geminiResp); err != nil {
			continue
		}
		parsedChunks++
		if len(geminiResp.Candidates) > 0 || geminiResp.UsageMetadata != nil {
			sawGeminiPayload = true
		}

		// 收集 usage
		if geminiResp.UsageMetadata != nil {
			totalUsage = *geminiResp.UsageMetadata
		}

		// 检查 prompt feedback 中的安全拦截
		if geminiResp.PromptFeedback != nil && geminiResp.PromptFeedback.BlockReason != "" {
			return fmt.Errorf("request blocked by Gemini safety filter: %s", geminiResp.PromptFeedback.BlockReason)
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

			if candidate.Content == nil {
				continue
			}

			for _, part := range candidate.Content.Parts {
				isThought := part.Thought != nil && *part.Thought
				// thoughtSignature 透传：签名附着在其来源 part 对应的 chunk 上
				//（thought 文本 → 消息级、functionCall → 工具级），part 未产出
				// chunk 时补发签名专用 chunk——Gemini 3 函数调用轮次强校验签名回传
				sig := part.ThoughtSignature
				sigAttached := false

				// 文本内容
				if part.Text != "" {
					if isThought {
						// thinking 内容 → reasoning_content
						if err := chunkWriter(newChunk(dto.Message{
							ReasoningContent: &part.Text,
							ThoughtSignature: sig,
						})); err != nil {
							return err
						}
						sigAttached = sig != ""
					} else {
						if err := chunkWriter(newChunk(dto.Message{
							Content:          part.Text,
							ThoughtSignature: sig,
						})); err != nil {
							return err
						}
						sigAttached = sig != ""
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
					if err := chunkWriter(newChunk(dto.Message{
						ToolCalls: []dto.ToolCall{{
							ID:   fmt.Sprintf("call_%s_%d", responseID, toolCallIdx),
							Type: "function",
							Function: dto.FunctionCall{
								Name:      part.FunctionCall.FunctionName,
								Arguments: string(argsJSON),
							},
							ThoughtSignature: sig,
						}},
					})); err != nil {
						return err
					}
					toolCallIdx++
					sigAttached = sigAttached || sig != ""
				}

				// 签名孤儿 part（无文本/函数调用等可附着内容）：补发签名专用 chunk
				if sig != "" && !sigAttached {
					if err := chunkWriter(newChunk(dto.Message{
						ThoughtSignature: sig,
					})); err != nil {
						return err
					}
				}
			}
		}
	}

	// 发送带 finish_reason 和 usage 的最终 chunk
	reason := finishReason
	if reason == "" {
		reason = "stop"
	}
	// 发出过 functionCall 时强制 tool_calls——Gemini 的 finishReason 为 STOP，
	// 直接映射成 stop 会让以 finish_reason 为判据的 agent 客户端不执行工具
	//（对齐非流式 gemini_to_openai_response 的强制修正）
	if toolCallIdx > 0 {
		reason = "tool_calls"
	}

	finalChunk := &dto.ChatCompletionStreamResponse{
		ID:      responseID,
		Object:  "chat.completion.chunk",
		Created: createdAt,
		Model:   modelName,
		Choices: []dto.StreamChoice{{
			Index:        0,
			FinishReason: &reason,
		}},
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

	if err := mismatchIfEmpty(); err != nil {
		return err
	}

	if err := chunkWriter(finalChunk); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	return nil
}
