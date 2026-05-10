package handler

import (
	"context"
	"net/http"

	"github.com/qianfree/team-api/relay/common"
)

// HandleAudioSpeech 处理 /v1/audio/speech 请求（TTS）
func HandleAudioSpeech(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleAudioTranscription 处理 /v1/audio/transcriptions 请求（STT）
func HandleAudioTranscription(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}

// HandleAudioTranslation 处理 /v1/audio/translations 请求
func HandleAudioTranslation(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	return RelayHandler(ctx, body, path, headers, rc, provider, billing)
}
