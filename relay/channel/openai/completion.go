package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relay/helper"
)

// handleCompletionNonStreamResponse 处理 Completions 非流式响应
func (a *Adaptor) handleCompletionNonStreamResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		if isUpstreamOpenAIError(body) {
			writeUpstreamErrorResponse(writer, resp.StatusCode, body)
			upstreamErr := constant.NewUpstreamError(resp.StatusCode, string(body), nil)
			upstreamErr.ResponseWritten = true
			return &common.Usage{}, upstreamErr
		}
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	var compResp dto.CompletionsResponse
	if err := json.Unmarshal(body, &compResp); err == nil {
		return &common.Usage{
			PromptTokens:           compResp.Usage.PromptTokens,
			CompletionTokens:       compResp.Usage.CompletionTokens,
			TotalTokens:            compResp.Usage.TotalTokens,
			PromptTokensDetails:    common.DtoTokenDetailsToCommon(compResp.Usage.PromptTokensDetails),
			CompletionTokenDetails: common.DtoTokenDetailsToCommon(compResp.Usage.CompletionTokenDetails),
		}, nil
	}

	return &common.Usage{}, nil
}
