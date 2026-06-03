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

// handleEmbeddingResponse 处理 Embeddings 非流式响应
func (a *Adaptor) handleEmbeddingResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read response body failed", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, constant.NewUpstreamError(resp.StatusCode, string(body), nil)
	}

	if info.ChannelMeta.IsModelMapped {
		body = helper.ReplaceModelName(body, info.OriginModelName)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(body)

	var embedResp dto.EmbeddingResponse
	if err := json.Unmarshal(body, &embedResp); err == nil {
		return &common.Usage{
			PromptTokens:           embedResp.Usage.PromptTokens,
			CompletionTokens:       embedResp.Usage.CompletionTokens,
			TotalTokens:            embedResp.Usage.TotalTokens,
			PromptTokensDetails:    common.DtoTokenDetailsToCommon(embedResp.Usage.PromptTokensDetails),
			CompletionTokenDetails: common.DtoTokenDetailsToCommon(embedResp.Usage.CompletionTokenDetails),
		}, nil
	}

	return &common.Usage{}, nil
}
