package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// Responses 生命周期动作（POST {id}/cancel 以 method 区分，其余按 HTTP method 透传）
const (
	responsesLifecycleCancel = "/cancel"
)

// HandleResponsesLifecycle 处理 Responses 生命周期端点：
// GET /v1/responses/{id}（retrieve）、POST /v1/responses/{id}/cancel、DELETE /v1/responses/{id}。
// 经 Redis 路由表（response_id → 渠道，Record 于直连转发时写入）还原原始渠道后透传上游。
// 无请求体、无计费（对齐异步任务查询端点先例）；错误以 *constant.RelayError 返回，
// 由调用方经 WriteRelayError 写为 OpenAI 格式。
func HandleResponsesLifecycle(ctx context.Context, method string, responseID string, rc *RelayContext, provider common.DataProvider) error {
	if responseID == "" {
		return constant.NewRequestError("response id is required", nil)
	}

	// 1. 查路由：miss（未记录/已过期/Redis 不可用）或渠道已不可物化均按 404 处理
	route, ok := common.DefaultResponseRouteStore.Lookup(ctx, rc.TenantID, responseID)
	if !ok {
		return &constant.RelayError{StatusCode: http.StatusNotFound, Message: "Response not found", Type: "invalid_request_error"}
	}

	selection, err := provider.MaterializeSelection(ctx, route.ChannelID, 0, route.ModelName)
	if err != nil {
		g.Log().Debugf(ctx, "[ResponsesLifecycle] materialize channel failed: responseID=%s channel=%d err=%v", responseID, route.ChannelID, err)
		return &constant.RelayError{StatusCode: http.StatusNotFound, Message: "Response not found", Type: "invalid_request_error"}
	}

	adaptor := channel.GetAdaptor(selection.ChannelType)
	if adaptor == nil {
		return constant.NewUpstreamError(http.StatusInternalServerError, fmt.Sprintf("unsupported channel type: %d", selection.ChannelType), nil)
	}

	info := buildLifecycleRelayInfo(rc, selection)
	info.RequestURLPath = "/responses/" + responseID
	// 守卫：路由记录只产生于 UpstreamSpeaksResponses 渠道，此处防御 chat-only 渠道
	//（GetRequestURL 会返回 /v1/chat/completions 导致拼错 URL）
	if !info.ChannelMeta.UpstreamSpeaksResponses() {
		return &constant.RelayError{StatusCode: http.StatusNotFound, Message: "Response not found", Type: "invalid_request_error"}
	}
	adaptor.Init(info)

	upstreamURL, err := buildLifecycleUpstreamURL(adaptor, info, responseID, method == http.MethodPost)
	if err != nil {
		return constant.NewUpstreamError(http.StatusInternalServerError, err.Error(), nil)
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, upstreamURL, nil)
	if err != nil {
		return constant.NewUpstreamError(http.StatusInternalServerError, "create lifecycle request failed", err)
	}
	if err := adaptor.SetupRequestHeader(httpReq.Header, info); err != nil {
		return constant.NewUpstreamError(http.StatusInternalServerError, "setup request header failed", err)
	}
	// 与标准转发链路一致：应用渠道级 Header Override
	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	client := common.NewPooledClient(info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode), info.ChannelMeta.Settings.UseProxy)
	resp, err := client.Do(httpReq)
	if err != nil {
		return constant.NewUpstreamError(http.StatusBadGateway, "upstream lifecycle request failed", err)
	}
	defer resp.Body.Close()

	// 原样透传上游响应（状态码 + Content-Type + body）
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		rc.Writer.Header().Set("Content-Type", ct)
	}
	rc.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(rc.Writer, resp.Body)

	// DELETE 成功后清理路由记录（上游已不可 retrieve）
	if method == http.MethodDelete && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		common.DefaultResponseRouteStore.Delete(ctx, rc.TenantID, responseID)
	}
	return nil
}

// buildLifecycleRelayInfo 构建生命周期转发的最小 RelayInfo（不经调度 FSM，
// 字段拷贝自路由记录物化的 ChannelSelection，照 buildRelayInfo 的 ChannelMeta 部分；
// RequestURLPath 由调用方按目标 responseID 覆写）
func buildLifecycleRelayInfo(rc *RelayContext, selection *common.ChannelSelection) *common.RelayInfo {
	return &common.RelayInfo{
		Context:         context.Background(),
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		ProjectID:       rc.ProjectID,
		RequestID:       rc.RequestID,
		RelayMode:       int(constant.RelayModeResponses),
		OriginModelName: selection.UpstreamModelName,
		StartTime:       time.Now(),
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         selection.ChannelID,
			ChannelType:       selection.ChannelType,
			ChannelName:       selection.ChannelName,
			BaseURL:           selection.BaseURL,
			ApiKey:            selection.ApiKey,
			UpstreamModelName: selection.UpstreamModelName,
			IsModelMapped:     selection.IsModelMapped,
			Settings:          selection.Settings,

			SupportsResponses: selection.SupportsResponses,
			ChatViaResponses:  selection.ChatViaResponses,
		},
	}
}

// buildLifecycleUpstreamURL 构建生命周期端点的上游 URL：
// 复用 adaptor.GetRequestURL（RelayModeResponses + UpstreamSpeaksResponses 时返回
// POST 基址，如 openai 的 /v1/responses、codex 的 /backend-api/codex/responses），
// 追加 /{responseID}（cancel 再追加 /cancel），对各 Responses 上游天然形状正确。
func buildLifecycleUpstreamURL(adaptor common.Adaptor, info *common.RelayInfo, responseID string, isCancel bool) (string, error) {
	base, err := adaptor.GetRequestURL(info)
	if err != nil {
		return "", err
	}
	url := base + "/" + responseID
	if isCancel {
		url += responsesLifecycleCancel
	}
	return url, nil
}
