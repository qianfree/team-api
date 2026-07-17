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

	"github.com/gogf/gf/v2/frame/g"
)

// handleRerankResponse 处理重排响应（JSON）
func handleRerankResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, constant.NewUpstreamError(resp.StatusCode, "read rerank response failed", err)
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

	// 提取 usage
	usage := extractRerankUsage(ctx, body)
	return usage, nil
}

// extractRerankUsage 从重排响应中提取 token 使用量
func extractRerankUsage(ctx context.Context, body []byte) *common.Usage {
	var rerankResp dto.RerankResponse
	if err := json.Unmarshal(body, &rerankResp); err != nil {
		g.Log().Warningf(ctx, "[OpenAI.extractRerankUsage] unmarshal rerank response failed: %v", err)
		return &common.Usage{}
	}

	promptTokens := rerankResp.Usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = rerankResp.Usage.TotalTokens
	}

	return &common.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  rerankResp.Usage.TotalTokens,
	}
}
