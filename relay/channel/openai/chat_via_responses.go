package openai

import (
	"context"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/common"
)

// handleChatViaResponsesNonStream 处理 Chat Completions via Responses API 的非流式响应
func (a *Adaptor) handleChatViaResponsesNonStream(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
) (*common.Usage, error) {
	g.Log().Infof(ctx, "[OpenAI] Chat via Responses bridge: non-stream mode")
	return HandleResponsesNonStreamToChat(ctx, resp, info, writer)
}

// handleChatViaResponsesStream 处理 Chat Completions via Responses API 的流式响应
func (a *Adaptor) handleChatViaResponsesStream(
	ctx context.Context,
	resp *http.Response,
	info *common.RelayInfo,
	writer http.ResponseWriter,
) (*common.Usage, error) {
	g.Log().Infof(ctx, "[OpenAI] Chat via Responses bridge: stream mode")
	return HandleResponsesStreamToChat(ctx, resp, info, writer)
}
