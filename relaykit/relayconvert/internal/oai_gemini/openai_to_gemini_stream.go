package oai_gemini

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

// OpenAIToGeminiStreamConverter OpenAI Chat 上游 SSE → Gemini 客户端流（流式响应侧，P2）。
// 移植宿主 relay/channel/openai/gemini_response.go 的 handleGeminiInboundStream，
// chunk 输出 *dto.GeminiChatResponse（宿主写出 `data:` 行格式，`data: [DONE]` 收尾由桥接写）。
//
// 与 legacy 的确定性差异（修复项，单测锁定）：
//   - 分片 tool arguments 聚合：Gemini 流式协议 functionCall.args 无增量语义（每次都应为
//     完整对象），legacy 对每个分片单独 unmarshal 几乎必败→产出垃圾 {name:"",args:{}} part；
//     修复为按 tc.Index 聚合 fragments，直到该工具的下一事件或收尾才发完整 part。
//
// legacy 语义保持：跳过条件（无 parts 且无 finish 的 chunk 丢弃；有 finish 无 parts 仍产
// candidate Parts=nil）；finishReason/usage 推迟到尾 chunk；candidates token 扣减 reasoning。
type OpenAIToGeminiStreamConverter struct{}

func (c *OpenAIToGeminiStreamConverter) ID() string {
	// 独立流式转换器：ID/From/To 表达自身真实流方向（openai→gemini）
	return relayconvert.ConverterOpenAIChatToGeminiContentStream
}

func (c *OpenAIToGeminiStreamConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToGeminiStreamConverter) To() types.RelayFormat {
	return types.RelayFormatGemini
}

// geminiToolFrag 流式聚合中的工具调用分片（按上游 tc.Index 归组，修复项）
type geminiToolFrag struct {
	name      string
	id        string
	argsParts []string
}

// ConvertStreamResponse 读取 chat SSE reader，经 chunkWriter 输出 *dto.GeminiChatResponse。
// 尾 chunk（含 finishReason 与 usageMetadata 的最终 candidate）后由宿主桥接写 [DONE]。
func (c *OpenAIToGeminiStreamConverter) ConvertStreamResponse(
	ctx context.Context, info convmeta.Meta, reader io.Reader, chunkWriter func(chunk any) error,
) error {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var (
		finishReason string
		usage        dto.UsageWithDetails
		// 工具调用聚合（修复项）：上游 index → 分片累积；顺序按首次出现
		toolFrags    []*geminiToolFrag
		toolFragIdx  = make(map[int]*geminiToolFrag)
		parsedChunks int  // 成功解析的 data 行数（假成功防护的诊断信息）
		sawChoices   bool // 是否出现过 choices —— chat 流的协议特征
	)

	// mismatchIfEmpty 假成功防护：整段上游流没有任何目标协议特征时报 ErrProtocolMismatch，
	// 由宿主桥接层按上游错误处理并置 StreamEndReasonError。绝不能静默收尾成空响应——
	// 客户端只会收到补发的终止事件而无任何内容，且该次请求在健康度上被记为成功、
	// 调度 FSM 失去换渠道机会。
	mismatchIfEmpty := func() error {
		if sawChoices || usage.TotalTokens > 0 || usage.PromptTokens > 0 {
			return nil
		}
		return fmt.Errorf("%w: %d chunks parsed, none contained choices", types.ErrProtocolMismatch, parsedChunks)
	}

	// flushToolFrags 把已聚合的完整 functionCall parts 追加进当前 candidate
	buildToolParts := func() []dto.GeminiPart {
		parts := make([]dto.GeminiPart, 0, len(toolFrags))
		for _, frag := range toolFrags {
			var args any
			joined := strings.Join(frag.argsParts, "")
			if joined != "" {
				if err := json.Unmarshal([]byte(joined), &args); err != nil {
					args = map[string]any{}
				}
			}
			parts = append(parts, dto.GeminiPart{FunctionCall: &dto.GeminiFunctionCall{
				ID: frag.id, FunctionName: frag.name, Arguments: args,
			}})
		}
		return parts
	}

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		line := scanner.Text()
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := extractGeminiSSEData(line)
		if data == "[DONE]" {
			break
		}

		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		parsedChunks++
		if len(chunk.Choices) > 0 {
			sawChoices = true
		}

		// usage 提取（任意 chunk 覆盖）
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}

		var geminiChunk dto.GeminiChatResponse
		for _, choice := range chunk.Choices {
			// n>1 的多 choice 流在 Gemini 单候选流无对应物，交错输出会损坏 parts——只处理首个 choice
			if choice.Index > 0 {
				continue
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				finishReason = *choice.FinishReason
			}
			var parts []dto.GeminiPart
			// thinking delta → thought part
			if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
				parts = append(parts, dto.GeminiPart{Text: *choice.Delta.ReasoningContent, Thought: boolPtrLocal(true)})
			}
			// 文本 delta（仅 string 形态）
			if text, ok := choice.Delta.Content.(string); ok && text != "" {
				parts = append(parts, dto.GeminiPart{Text: text})
			}
			// 工具 delta：聚合分片（修复项），不即时产 part
			for _, tc := range choice.Delta.ToolCalls {
				frag, ok := toolFragIdx[tc.Index]
				if !ok {
					frag = &geminiToolFrag{id: tc.ID, name: tc.Function.Name}
					toolFrags = append(toolFrags, frag)
					toolFragIdx[tc.Index] = frag
				}
				if tc.Function.Name != "" {
					frag.name = tc.Function.Name
				}
				if tc.ID != "" {
					frag.id = tc.ID
				}
				if tc.Function.Arguments != "" {
					frag.argsParts = append(frag.argsParts, tc.Function.Arguments)
				}
			}
			// 跳过条件（legacy 口径）：无即时 parts、无 finish、无已聚合工具 → 整 chunk 丢弃
			if len(parts) == 0 && choice.FinishReason == nil && len(toolFrags) == 0 {
				continue
			}
			if len(parts) > 0 {
				geminiChunk.Candidates = append(geminiChunk.Candidates, dto.GeminiCandidate{
					Index:   choice.Index,
					Content: &dto.GeminiContent{Role: "model", Parts: parts},
				})
			}
		}
		if len(geminiChunk.Candidates) > 0 {
			if err := chunkWriter(&geminiChunk); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return fmt.Errorf("stream scanner error: %w", err)
	}

	// 尾 chunk：聚合的工具 parts + finishReason + usageMetadata（legacy 推迟到收尾的口径）
	tail := dto.GeminiChatResponse{
		Candidates: []dto.GeminiCandidate{{
			Content:      &dto.GeminiContent{Role: "model", Parts: buildToolParts()},
			FinishReason: reasonmap.OpenAIFinishReasonToGeminiFinishReason(finishReason),
		}},
	}
	// 2f0cc01 扣减口径（流式拷贝）：Gemini candidates 不含 thoughts，OpenAI completion 已含
	candidatesTokens := usage.CompletionTokens
	if usage.CompletionTokenDetails != nil && usage.CompletionTokenDetails.ReasoningTokens > 0 {
		candidatesTokens -= usage.CompletionTokenDetails.ReasoningTokens
		if candidatesTokens < 0 {
			candidatesTokens = 0
		}
	}
	um := &dto.GeminiUsageMetadata{
		PromptTokenCount:     usage.PromptTokens,
		CandidatesTokenCount: candidatesTokens,
		TotalTokenCount:      usage.TotalTokens,
	}
	if usage.TotalTokens == 0 {
		um.TotalTokenCount = usage.PromptTokens + usage.CompletionTokens
	}
	if usage.PromptTokensDetails != nil {
		um.CachedContentTokenCount = usage.PromptTokensDetails.CachedTokens
	}
	if usage.CompletionTokenDetails != nil {
		um.ThoughtsTokenCount = usage.CompletionTokenDetails.ReasoningTokens
	}
	// legacy 口径：usage 有值时才附 UsageMetadata
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		tail.UsageMetadata = um
	}
	if err := mismatchIfEmpty(); err != nil {
		return err
	}
	return chunkWriter(&tail)
}

// extractGeminiSSEData 从 SSE data 行提取数据部分。
func extractGeminiSSEData(line string) string {
	data := strings.TrimPrefix(line, "data:")
	return strings.TrimSpace(data)
}
