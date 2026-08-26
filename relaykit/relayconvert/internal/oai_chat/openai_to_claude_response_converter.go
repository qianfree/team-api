package oai_chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/reasonmap"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// OpenAIToClaudeResponseConverter OpenAI Chat 上游 → Claude 客户端（非流式响应侧，P1-B）。
// 逐行移植宿主 relay/channel/openai/claude_response.go 的 openAIToClaudeResponse，
// 挂在 spec A（ConverterClaudeMessagesToOpenAIChat）的 Resp 侧（方向与请求相反）。
//
// legacy 怪癖清单（保持勿改）：
//   - 只取 Choices[0]；块序 thinking→text→tool_use（thinking 块 signature 承载
//     Gemini thoughtSignature 往返，见 Message/ToolCall.ThoughtSignature）
//   - Content 非 string（数组形态）不提取任何 text（断言失败即无）
//   - ToolCalls Arguments unmarshal 失败→{}；content 空时补空 text 块；空 choices 仍产完整骨架
//   - finish_reason 用 LegacySemantics 精确复刻（不 ToLower、空串→end_turn）——
//     与本包既有 OpenAIFinishReasonToClaudeStopReason 语义不同，勿混用
//   - ID = msg_<RequestID>（合成，经能力接口）；Model = resp.Model，映射渠道→OriginModelName
//   - usage：InputTokens = Prompt - Cached（判空、<0 归 0）、CacheRead = Cached、Output 直传
type OpenAIToClaudeResponseConverter struct{}

func (c *OpenAIToClaudeResponseConverter) ID() string {
	return relayconvert.ConverterClaudeMessagesToOpenAIChat
}

func (c *OpenAIToClaudeResponseConverter) From() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *OpenAIToClaudeResponseConverter) To() types.RelayFormat {
	return types.RelayFormatClaude
}

// ConvertResponse 入参断言 *dto.ChatCompletionResponse，输出 *dto.ClaudeResponse。
func (c *OpenAIToClaudeResponseConverter) ConvertResponse(
	ctx context.Context, info convmeta.Meta, response any,
) (any, *dto.UsageWithDetails, error) {
	resp, ok := response.(*dto.ChatCompletionResponse)
	if !ok {
		return nil, nil, fmt.Errorf("expected *dto.ChatCompletionResponse, got %T", response)
	}
	return buildClaudeResponseFromOpenAI(info, resp), nil, nil
}

func buildClaudeResponseFromOpenAI(info convmeta.Meta, openaiResp *dto.ChatCompletionResponse) *dto.ClaudeResponse {
	content := make([]dto.ClaudeContentBlock, 0)
	var textParts []string
	var thinkingParts []string
	var toolCalls []dto.ClaudeContentBlock
	// thoughtSignature 透传：消息级优先，其次首个非空的工具级签名——
	// 附着到 thinking 块的 signature 字段（Claude Code 回传 thinking 块实现签名往返）
	thoughtSignature := ""

	if len(openaiResp.Choices) > 0 {
		choice := openaiResp.Choices[0]
		thoughtSignature = choice.Message.ThoughtSignature

		// 提取文本（仅 string 形态——legacy 口径）
		if text, ok := choice.Message.Content.(string); ok && text != "" {
			textParts = append(textParts, text)
		}

		// 提取思维内容
		if choice.Message.ReasoningContent != nil && *choice.Message.ReasoningContent != "" {
			thinkingParts = append(thinkingParts, *choice.Message.ReasoningContent)
		}

		// 提取工具调用
		for _, tc := range choice.Message.ToolCalls {
			var inputObj any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputObj); err != nil {
				inputObj = map[string]any{}
			}
			if thoughtSignature == "" && tc.ThoughtSignature != "" {
				thoughtSignature = tc.ThoughtSignature
			}
			toolCalls = append(toolCalls, dto.ClaudeContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: inputObj,
			})
		}
	}

	// 块序：thinking → text → tool_use（Claude 语义）
	for i, thinking := range thinkingParts {
		t := thinking
		block := dto.ClaudeContentBlock{
			Type:     "thinking",
			Thinking: &t,
		}
		// 签名附着到末个 thinking 块
		if i == len(thinkingParts)-1 {
			block.Signature = thoughtSignature
		}
		content = append(content, block)
	}
	// 有签名但无 thinking 块（如未开 includeThoughts 的 Gemini 3 函数调用）：补空 thinking 块承载
	if thoughtSignature != "" && len(thinkingParts) == 0 {
		content = append(content, dto.ClaudeContentBlock{
			Type:      "thinking",
			Thinking:  strPtrToLocal(""),
			Signature: thoughtSignature,
		})
	}
	for _, text := range textParts {
		t := text
		content = append(content, dto.ClaudeContentBlock{
			Type: "text",
			Text: &t,
		})
	}
	content = append(content, toolCalls...)

	if len(content) == 0 {
		content = append(content, dto.ClaudeContentBlock{Type: "text", Text: strPtrToLocal("")})
	}

	modelName := openaiResp.Model
	if convmeta.ModelNameMappedOf(info) {
		modelName = info.GetOriginModelName()
	}

	stopReason := "end_turn"
	if len(openaiResp.Choices) > 0 {
		stopReason = reasonmap.OpenAIFinishReasonToClaudeLegacySemantics(openaiResp.Choices[0].FinishReason)
	}

	// OpenAI 的 prompt_tokens 已含 cached（子集语义），Claude 的 input_tokens 不含 cache_read，
	// 需扣减后映射；PromptTokensDetails 可能为 nil，必须判空
	cachedTokens := 0
	if openaiResp.Usage.PromptTokensDetails != nil {
		cachedTokens = openaiResp.Usage.PromptTokensDetails.CachedTokens
	}
	inputTokens := openaiResp.Usage.PromptTokens - cachedTokens
	if inputTokens < 0 {
		inputTokens = 0
	}

	return &dto.ClaudeResponse{
		ID:           fmt.Sprintf("msg_%s", convmeta.RequestIDOf(info)),
		Type:         "message",
		Role:         "assistant",
		Content:      content,
		StopReason:   stopReason,
		StopSequence: nil,
		Model:        modelName,
		Usage: &dto.ClaudeUsage{
			InputTokens:          inputTokens,
			OutputTokens:         openaiResp.Usage.CompletionTokens,
			CacheReadInputTokens: cachedTokens,
		},
	}
}

func strPtrToLocal(v string) *string { return &v }
