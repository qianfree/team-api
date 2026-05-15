package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// StreamHandler 处理 OpenAI SSE 流式响应（goroutine + channel 解耦读写）
func StreamHandler(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	var totalUsage dto.UsageWithDetails
	var responseTextBuf strings.Builder

	helper.StreamScannerHandler(ctx, resp, info, writer, func(data string, sr *helper.StreamResult) {
		if info.ChannelMeta.IsModelMapped {
			data = string(replaceModelName([]byte(data), info.OriginModelName))
		}

		var streamResp dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
			if streamResp.Usage != nil {
				totalUsage.PromptTokens = streamResp.Usage.PromptTokens
				totalUsage.CompletionTokens = streamResp.Usage.CompletionTokens
				totalUsage.TotalTokens = streamResp.Usage.TotalTokens
				if streamResp.Usage.PromptTokensDetails != nil {
					totalUsage.PromptTokensDetails = streamResp.Usage.PromptTokensDetails
				}
				if streamResp.Usage.CompletionTokenDetails != nil {
					totalUsage.CompletionTokenDetails = streamResp.Usage.CompletionTokenDetails
				}
			}
			for _, choice := range streamResp.Choices {
				if choice.Delta.Content != nil {
					if text, ok := choice.Delta.Content.(string); ok {
						responseTextBuf.WriteString(text)
					}
				}
				if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
					responseTextBuf.WriteString(*choice.Delta.ReasoningContent)
				}
			}
		}

		if err := helper.WriteSSEData(writer, data); err != nil {
			sr.Stop(fmt.Errorf("write sse data failed: %w", err))
			return
		}
	})

	// 根据流结束原因决定返回值
	// relay_handler.go 依赖返回的 error 判断是否需要部分结算或退费
	usage := &common.Usage{
		PromptTokens:           totalUsage.PromptTokens,
		CompletionTokens:       totalUsage.CompletionTokens,
		TotalTokens:            totalUsage.TotalTokens,
		PromptTokensDetails:    common.DtoTokenDetailsToCommon(totalUsage.PromptTokensDetails),
		CompletionTokenDetails: common.DtoTokenDetailsToCommon(totalUsage.CompletionTokenDetails),
		CacheIncludedInPrompt:  true,
	}

	// 如果上游未返回 usage，用累积的响应文本估算
	if usage.TotalTokens == 0 && responseTextBuf.Len() > 0 {
		estimated := responseTextBuf.Len() / 4
		usage.CompletionTokens = estimated
		usage.TotalTokens = usage.PromptTokens + estimated
	}

	// 正常结束：发送 [DONE] 通知客户端流结束。
	// 客户端依赖 data: [DONE] 判断流结束而非连接关闭；
	// 弱网环境下后处理耗时较长，不发 [DONE] 客户端会一直等待直到网络超时。
	if info.StreamStatus == nil || !info.StreamStatus.IsPartialStreamEnd() {
		_ = helper.WriteSSEData(writer, "[DONE]")
	}

	if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
		return usage, common.ErrStreamInterrupted
	}
	return usage, nil
}

// StreamHandlerForCompletions 处理 Completions 流式响应（goroutine + channel 解耦读写）
func StreamHandlerForCompletions(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	var totalUsage dto.UsageWithDetails
	var responseTextBuf strings.Builder

	helper.StreamScannerHandler(ctx, resp, info, writer, func(data string, sr *helper.StreamResult) {
		if info.ChannelMeta.IsModelMapped {
			data = string(replaceModelName([]byte(data), info.OriginModelName))
		}

		var streamResp dto.CompletionsStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err == nil {
			if streamResp.Usage != nil {
				totalUsage.PromptTokens = streamResp.Usage.PromptTokens
				totalUsage.CompletionTokens = streamResp.Usage.CompletionTokens
				totalUsage.TotalTokens = streamResp.Usage.TotalTokens
				if streamResp.Usage.PromptTokensDetails != nil {
					totalUsage.PromptTokensDetails = streamResp.Usage.PromptTokensDetails
				}
				if streamResp.Usage.CompletionTokenDetails != nil {
					totalUsage.CompletionTokenDetails = streamResp.Usage.CompletionTokenDetails
				}
			}
			for _, choice := range streamResp.Choices {
				responseTextBuf.WriteString(choice.Text)
			}
		}

		if err := helper.WriteSSEData(writer, data); err != nil {
			sr.Stop(fmt.Errorf("write sse data failed: %w", err))
			return
		}
	})

	usage := &common.Usage{
		PromptTokens:           totalUsage.PromptTokens,
		CompletionTokens:       totalUsage.CompletionTokens,
		TotalTokens:            totalUsage.TotalTokens,
		PromptTokensDetails:    common.DtoTokenDetailsToCommon(totalUsage.PromptTokensDetails),
		CompletionTokenDetails: common.DtoTokenDetailsToCommon(totalUsage.CompletionTokenDetails),
		CacheIncludedInPrompt:  true,
	}

	if usage.TotalTokens == 0 && responseTextBuf.Len() > 0 {
		estimated := responseTextBuf.Len() / 4
		usage.CompletionTokens = estimated
		usage.TotalTokens = usage.PromptTokens + estimated
	}

	if info.StreamStatus == nil || !info.StreamStatus.IsPartialStreamEnd() {
		_ = helper.WriteSSEData(writer, "[DONE]")
	}

	if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
		return usage, common.ErrStreamInterrupted
	}
	return usage, nil
}
