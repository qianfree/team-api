package dify_chat

import (
	"context"
	"fmt"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"
	"github.com/qianfree/team-api/relaykit/types"
)

// DifyToOpenAIResponseConverter 将 Dify blocking 响应转换为 OpenAI Chat Completions 响应。
type DifyToOpenAIResponseConverter struct{}

func (c *DifyToOpenAIResponseConverter) ID() string {
	return relayconvert.ResponseConverterDifyChatToOAIChat
}

func (c *DifyToOpenAIResponseConverter) From() types.RelayFormat {
	return types.RelayFormatDify
}

func (c *DifyToOpenAIResponseConverter) To() types.RelayFormat {
	return types.RelayFormatOpenAI
}

func (c *DifyToOpenAIResponseConverter) Quality() relayconvert.ResponseConverterQuality {
	return relayconvert.ResponseConverterQualityFair
}

// ConvertResponse 将 Dify blocking 响应转换为 OpenAI 非流式响应。
// 注意：方法签名返回 (any, error)，注册时由 register 包适配闭包补 nil Usage。
func (c *DifyToOpenAIResponseConverter) ConvertResponse(
	ctx context.Context,
	info convmeta.Meta,
	response any,
) (any, error) {
	difyResp, ok := response.(*dto.DifyBlockingResponse)
	if !ok {
		return nil, fmt.Errorf("expected *dto.DifyBlockingResponse, got %T", response)
	}

	modelName := ""
	if info != nil {
		modelName = info.GetOriginModelName()
	}

	usage := dto.UsageWithDetails{
		PromptTokens:     difyResp.Metadata.Usage.PromptTokens,
		CompletionTokens: difyResp.Metadata.Usage.CompletionTokens,
		TotalTokens:      difyResp.Metadata.Usage.TotalTokens,
	}
	// Dify 未返回用量时，用 Answer 长度粗估 completion tokens
	if usage.TotalTokens == 0 {
		usage.CompletionTokens = estimateTokens(difyResp.Answer)
		usage.TotalTokens = usage.CompletionTokens
	}

	openaiResp := &dto.ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%d", getCurrentTimestamp()),
		Object:  "chat.completion",
		Created: 0,
		Model:   modelName,
		Choices: []dto.Choice{{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: difyResp.Answer,
			},
			FinishReason: "stop",
		}},
		Usage: usage,
	}

	return openaiResp, nil
}

// estimateTokens 粗略估算 token 数（4 字符 ≈ 1 token），与旧实现 helper.EstimateTokens 口径一致。
func estimateTokens(s string) int {
	return len(s) / 4
}
