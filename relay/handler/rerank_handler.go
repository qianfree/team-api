package handler

import (
	"context"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
)

// HandleRerank 处理 /v1/rerank 请求
func HandleRerank(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}
