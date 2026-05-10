package handler

import (
	"context"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
)

// HandleModerations 处理 /v1/moderations 请求
func HandleModerations(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleImagesEdits 处理 /v1/images/edits 请求
func HandleImagesEdits(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}
