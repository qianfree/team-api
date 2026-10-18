package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"

	"github.com/qianfree/team-api/internal/dispatchadapter"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

// websocketUpgrader HTTP → WebSocket 升级器
var websocketUpgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"},
	CheckOrigin:  checkRealtimeOrigin,
}

// checkRealtimeOrigin 校验 WebSocket 握手来源，防跨站 WebSocket 劫持（CSWSH，B15）。
// 策略：
//   - 无 Origin 头：放行。Realtime API 的主要客户端是原生 SDK/CLI/服务端，不发送 Origin；
//     浏览器才强制带 Origin，据此区分。
//   - 同源（Origin.host == 请求 Host）：放行。
//   - 命中 REALTIME_ALLOWED_ORIGINS（逗号分隔的可信来源，如租户控制台/playground 域名）：放行。
//   - 其余跨源浏览器请求：拒绝。
func checkRealtimeOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // 非浏览器客户端
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}

	if strings.EqualFold(u.Host, r.Host) {
		return true // 同源
	}

	for _, allowed := range strings.Split(os.Getenv("REALTIME_ALLOWED_ORIGINS"), ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if strings.EqualFold(origin, allowed) || strings.EqualFold(u.Host, allowed) {
			return true
		}
	}

	return false
}

// RealtimeContext Realtime 请求上下文
type RealtimeContext struct {
	TenantID        int64
	UserID          int64
	ApiKeyID        int64
	ProjectID       int64 // 通过 API Key 关联的项目 ID
	RequestID       string
	ClientIP        string
	Scope           string
	KeyRateLimitQps int
	KeyConcurrency  int
	KeyIpWhitelist  string
	KeyTotalQuota   float64
	KeyUsedQuota    float64

	// ResolvedModel 连接最终使用的模型名（URL query 或首条 WS 消息解析所得），
	// 由 handler 回写，供上层包装器记审计日志
	ResolvedModel string
}

// HandleRealtime 处理 /v1/realtime WebSocket 请求
func HandleRealtime(w http.ResponseWriter, r *http.Request, rc *RealtimeContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	ctx := r.Context()

	// 0. 前置检查（scope / IP 白名单 / QPS 限流）：只依赖认证上下文，放在 WebSocket
	// 升级之前——升级后连接已被劫持，无法再写标准 HTTP 状态码；前置后被拒请求能拿到
	// 正常的 401/429 响应，也省掉无谓的升级握手和首消息读取。
	// 注意：上层调用方（internal/handler/relay）对 realtime 的返回错误只记日志、不写响应
	//（因升级后的错误无法写回），故升级前的拒绝必须在此处自行写回 HTTP 错误响应，
	// 否则客户端会收到空 body 的 200。
	billingResult := &BillingResult{}
	if billing != nil {
		if !billing.CheckScope(rc.Scope, "realtime") {
			err := constant.NewAuthError("API key scope denied")
			WriteRelayError(w, err)
			return nil, billingResult, err
		}
		if !billing.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
			err := constant.NewAuthError("IP address is not allowed")
			WriteRelayError(w, err)
			return nil, billingResult, err
		}
		allowed, limitLevel, limit, remaining, resetAt := billing.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, rc.KeyRateLimitQps)
		if !allowed {
			err := &RelayErrorWithRateLimit{
				StatusCode: 429,
				Message:    fmt.Sprintf("rate limit exceeded at %s level", limitLevel),
				LimitLevel: limitLevel,
				Remaining:  remaining,
				ResetAt:    resetAt,
			}
			WriteRelayError(w, err)
			return nil, billingResult, err
		}
		billingResult.RateLimitInfo = &common.RateLimitInfo{
			Limit:     limit,
			Remaining: remaining,
			ResetAt:   resetAt,
		}
	}

	// 1. WebSocket 升级
	clientConn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		g.Log().Errorf(ctx, "[HandleRealtime] WebSocket upgrade failed: %v", err)
		return nil, nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}
	defer clientConn.Close()

	// 2. 确定模型名：优先 URL query 的 model 参数（官方 SDK 连接约定
	// wss://host/v1/realtime?model=xxx，客户端连接后不主动发含 model 的消息，
	// 等服务端 session.created），缺失时回退读取首条 WS 消息提取。
	// query 命中时不再阻塞读首条消息，所有客户端消息经 Proxy 循环正常转发。
	modelName := r.URL.Query().Get("model")
	var firstMsg []byte
	if modelName == "" {
		_, firstMsg, err = clientConn.ReadMessage()
		if err != nil {
			return nil, nil, fmt.Errorf("read first websocket message failed: %w", err)
		}

		var firstEvent map[string]json.RawMessage
		if err := json.Unmarshal(firstMsg, &firstEvent); err != nil {
			return nil, nil, fmt.Errorf("parse first websocket message failed: %w", err)
		}

		modelName = extractModelFromEvent(firstEvent)
		if modelName == "" {
			errMsg, _ := json.Marshal(map[string]any{
				"type":  "error",
				"error": map[string]string{"message": "model is required in the URL query or the first message"},
			})
			_ = clientConn.WriteMessage(websocket.TextMessage, errMsg)
			return nil, nil, fmt.Errorf("model not found in websocket URL query or first message")
		}
	}
	rc.ResolvedModel = modelName

	// 3. 验证模型
	_, _, err = provider.GetModelMapping(ctx, modelName)
	if err != nil {
		return nil, nil, fmt.Errorf("model not found: %s", modelName)
	}

	// 3.1 检查成员模型范围
	if allowed, err := provider.CheckMemberModelAccess(ctx, rc.TenantID, rc.UserID, modelName); err != nil {
		return nil, nil, err
	} else if !allowed {
		return nil, nil, constant.NewAuthError("model not allowed for this member")
	}
	if allowed, err := provider.CheckApiKeyModelAccess(ctx, rc.ApiKeyID, modelName); err != nil {
		return nil, nil, err
	} else if !allowed {
		return nil, nil, constant.NewAuthError("model not allowed for this API key")
	}

	// 4. 渠道选择（新调度引擎；websocket 长连接由续期器保活租约）
	enabled, channelScope, err := provider.CheckTenantModelAccess(ctx, rc.TenantID, modelName)
	if err != nil {
		return nil, nil, constant.NewChannelError("no available channel for model: "+modelName, err)
	}
	if !enabled {
		return nil, nil, constant.NewChannelError("no available channel for model: "+modelName, common.ErrTenantModelNotEnabled)
	}
	sess := dispatchadapter.Coordinator(ctx).Route(ctx, dispatch.RequestProfile{
		RequestID: rc.RequestID,
		TenantID:  rc.TenantID,
		UserID:    rc.UserID,
		APIKeyID:  rc.ApiKeyID,
		Model:     modelName,
		Scope:     channelScope,
		Replay:    dispatch.ReplayCostly,
		Signals:   dispatch.SessionSignals{HeaderSessionID: r.Header.Get("X-Session-Id")},
		Policy:    dispatchadapter.TenantRoutingPolicy(ctx, rc.TenantID),
	})
	defer sess.Finish(context.WithoutCancel(ctx), false, 0)

	var selection *common.ChannelSelection
	for {
		d := sess.Next(ctx)
		if d == nil {
			monitor.TrackDispatchNoCandidate()
			return nil, nil, constant.NewChannelError("no available channel for model: "+modelName, common.ErrChannelUnavailable)
		}
		monitor.TrackDispatchSelection(string(d.Reason), string(d.Channel.Tier), string(d.SessionKey.Source))
		sel, mErr := provider.MaterializeSelection(ctx, d.Channel.ID, d.KeyID, modelName)
		if mErr != nil {
			if decision := reportMaterializeFailure(ctx, sess, mErr); decision == dispatch.DecisionAbort {
				return nil, nil, constant.NewChannelError("no available channel for model: "+modelName, mErr)
			}
			continue
		}
		sel.SelectionReason = string(d.Reason)
		selection = sel
		break
	}
	leaseRefresh := startDispatchLeaseRefresher(selection.ChannelID, rc.RequestID)
	defer leaseRefresh.Stop()
	realtimeStart := time.Now()

	// 5. 构造 RelayInfo
	info := &common.RelayInfo{
		Context:         ctx,
		TenantID:        rc.TenantID,
		UserID:          rc.UserID,
		ApiKeyID:        rc.ApiKeyID,
		RequestID:       rc.RequestID,
		RelayMode:       int(constant.RelayModeRealtime),
		OriginModelName: modelName,
		RequestURLPath:  "/realtime",
		RequestHeaders:  r.Header,
		StartTime:       time.Now(),
		ProjectID:       rc.ProjectID,
		InboundFormat:   constant.RelayFormatOpenAI,
		ClientFormat:    constant.RelayFormatOpenAI,
		ChannelMeta: &common.ChannelMeta{
			ChannelID:         selection.ChannelID,
			ChannelType:       selection.ChannelType,
			ChannelName:       selection.ChannelName,
			BaseURL:           selection.BaseURL,
			ApiKey:            selection.ApiKey,
			UpstreamModelName: selection.UpstreamModelName,
			IsModelMapped:     selection.IsModelMapped,
			Settings:          selection.Settings,
		},
		ClientConn: clientConn,
	}

	// 6. 获取适配器
	adaptor := channel.GetAdaptor(selection.ChannelType)
	if adaptor == nil {
		return nil, nil, fmt.Errorf("unsupported channel type for realtime: %d", selection.ChannelType)
	}
	adaptor.Init(info)

	// 7. 预扣费用（Realtime 使用 0 input tokens 作为初始估算）
	var preDeductAmount float64
	if billing != nil {
		if !billing.AcquireConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, modelName) {
			return nil, billingResult, constant.NewRateLimitError("concurrent request limit exceeded")
		}
		defer billing.ReleaseConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, modelName)

		if !billing.AcquireApiKeyConcurrent(ctx, rc.ApiKeyID, rc.KeyConcurrency) {
			return nil, billingResult, constant.NewRateLimitError("API key concurrent request limit exceeded")
		}
		defer billing.ReleaseApiKeyConcurrent(ctx, rc.ApiKeyID)

		amt, billErr := billing.PreDeduct(ctx, rc.TenantID, modelName, 0, 0, false, rc.RequestID)
		if billErr != nil {
			// 未配价模型 fail-closed：返回明确的请求错误，而非误导性的"余额不足"
			if errors.Is(billErr, common.ErrModelPricingNotConfigured) {
				return nil, nil, constant.NewRequestError("model pricing not configured: "+modelName, billErr)
			}
			return nil, nil, constant.NewQuotaError("insufficient balance", billErr)
		}
		preDeductAmount = amt
		// 成员/Key 额度检查只做这一次（带实际预扣额），与 RelayHandler 一致：
		// 预扣前的 Check(0) 快速闸门对全量请求多付 Redis 往返，收益不成比例，已移除。
		if err := billing.CheckApiKeyQuota(ctx, rc.ApiKeyID, amt); err != nil {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, amt)
			return nil, billingResult, constant.NewQuotaError("API key quota exceeded", err)
		}
		if err := billing.CheckMemberQuota(ctx, rc.TenantID, rc.UserID, amt); err != nil {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, amt)
			return nil, billingResult, constant.NewQuotaError("member quota exceeded", err)
		}
	}

	billingResult.PreDeductAmount = preDeductAmount

	// 8. 建立 Realtime 代理
	proxy := openai.NewRealtimeProxy(info)
	if err := proxy.DialUpstream(); err != nil {
		// 拨号失败上报健康（绑定不删除，由守卫自然失效）；realtime 不做跨渠道重试，保持原语义
		_, _ = sess.Report(ctx, 0, err, deliveryStateOfRequestErr(err), float64(time.Since(realtimeStart).Milliseconds()), 0)
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		recordRealtimeUsage(provider, rc, r, selection, modelName, nil, nil, err, int(time.Since(realtimeStart).Milliseconds()))
		return nil, billingResult, err
	}
	defer proxy.Close()

	// 把首条消息转发到上游：仅在模型取自首条消息时存在缓存的 firstMsg
	//（此时该消息已被读出、不会经 Proxy 循环）；模型来自 URL query 时
	// 客户端消息全部由 Proxy 循环正常转发，无需补发。
	if firstMsg != nil {
		if err := proxy.GetTargetConn().WriteMessage(websocket.TextMessage, firstMsg); err != nil {
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			recordRealtimeUsage(provider, rc, r, selection, modelName, nil, nil, err, int(time.Since(realtimeStart).Milliseconds()))
			return nil, billingResult, err
		}
	}

	// 9. 启动双向代理
	usage, proxyErr := proxy.Proxy(ctx)
	if proxyErr == nil {
		// 成功：绑定续期 + 健康上报由 Finish 完成
		sess.Finish(context.WithoutCancel(ctx), true, float64(time.Since(realtimeStart).Milliseconds()))
	} else {
		// 会话中断：响应已开始，不可重试，仅上报健康
		_, _ = sess.Report(context.WithoutCancel(ctx), 0, proxyErr, dispatch.DeliveryResponseStarted,
			float64(time.Since(realtimeStart).Milliseconds()), 0)
	}

	// 10. 结算费用 + 记录用量（与 HTTP 管线对齐：结算与 bil_usage_logs 是两条独立
	// 写入链路，realtime 此前只结算不落用量日志，租户账单与用量报表对不齐）
	var settleResult *common.SettlementResult
	if billing != nil && preDeductAmount > 0 {
		if proxyErr != nil {
			streamUsage := usage
			if streamUsage == nil {
				streamUsage = &common.Usage{}
			}
			settleResult, _ = billing.SettleStreamInterrupted(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
				modelName, rc.RequestID, "realtime", streamUsage, preDeductAmount, rc.ProjectID, info.StartTime)
			if settleResult != nil && settleResult.ActualCost > 0 && !settleResult.DuplicateSkip {
				billing.IncrMemberQuotaUsed(ctx, rc.TenantID, rc.UserID, settleResult.ActualCost)
				billing.IncrApiKeyQuotaUsed(ctx, rc.ApiKeyID, settleResult.ActualCost)
			}
		} else if usage != nil {
			// 走 SettleWithUsage 完整 Usage 结算：realtime usage 携带缓存明细
			//（input_token_details.cached_tokens），plain Settle 只取 input/output 会把缓存按全价计费
			settleResult = billing.SettleWithUsage(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
				modelName, rc.RequestID, "realtime", usage, preDeductAmount, info)
			if settleResult != nil && settleResult.ActualCost > 0 && !settleResult.DuplicateSkip {
				billing.IncrMemberQuotaUsed(ctx, rc.TenantID, rc.UserID, settleResult.ActualCost)
				billing.IncrApiKeyQuotaUsed(ctx, rc.ApiKeyID, settleResult.ActualCost)
			}
		}
	}
	recordRealtimeUsage(provider, rc, r, selection, modelName, usage, settleResult, proxyErr, int(time.Since(realtimeStart).Milliseconds()))

	return usage, billingResult, proxyErr
}

// recordRealtimeUsage 记录 Realtime 会话的用量日志。成功、中断（含拨号/首条消息
// 转发失败）均落库：结算已扣费的会话若无量记录，钱包流水与用量报表对不齐。
// token 口径与 settleSuccessfulRequest 一致（input 含缓存、音频 token 分离）。
func recordRealtimeUsage(
	provider common.DataProvider,
	rc *RealtimeContext,
	r *http.Request,
	selection *common.ChannelSelection,
	modelName string,
	usage *common.Usage,
	settleResult *common.SettlementResult,
	proxyErr error,
	latencyMs int,
) {
	if provider == nil {
		return
	}
	if usage == nil {
		usage = &common.Usage{}
	}

	totalInput := usage.TotalInputTokens()
	totalTokens := totalInput + usage.CompletionTokens
	if totalTokens == 0 {
		totalTokens = usage.TotalTokens
	}

	record := &common.UsageRecord{
		TenantID:         rc.TenantID,
		UserID:           rc.UserID,
		ApiKeyID:         rc.ApiKeyID,
		ProjectID:        rc.ProjectID,
		ChannelID:        selection.ChannelID,
		ChannelName:      selection.ChannelName,
		ChannelType:      selection.ChannelType,
		ModelName:        modelName,
		RequestedModel:   modelName,
		UpstreamModel:    selection.UpstreamModelName,
		RelayMode:        int(constant.RelayModeRealtime),
		PromptTokens:     totalInput,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      totalTokens,
		CachedTokens:     tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedTokens }),
		AudioInputTokens: tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.AudioTokens }),
		AudioOutputTokens: tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int {
			return d.AudioTokens
		}),
		LatencyMs:       float64(latencyMs),
		IsStream:        true,
		RequestType:     2, // 流式（与 requestType 一致）
		RequestID:       rc.RequestID,
		InboundEndpoint: "/realtime",
		UserAgent:       r.Header.Get("User-Agent"),
		ClientIP:        rc.ClientIP,
	}

	if proxyErr == nil {
		record.Success = true
		record.Status = "success"
	} else {
		record.Success = false
		record.Status = "error"
		record.ErrorMessage = helper.SafeUpstreamErrorMessage(proxyErr)
	}

	if settleResult != nil {
		record.TotalCost = settleResult.BaseCost
		record.ActualCost = settleResult.ActualCost
		record.Currency = "USD"
		record.PreDeductAmount = settleResult.PreDeductAmount
		record.RefundAmount = settleResult.RefundAmount
		record.SupplementAmount = settleResult.SupplementAmount
		record.BillingSnapshot = settleResult.BillingSnapshot
		record.BillingSummary = settleResult.BillingSummary
		record.BillingMode = settleResult.BillingMode
		record.BillingSource = settleResult.BillingSource
		record.RateMultiplier = settleResult.RateMultiplier
		record.InputCost = settleResult.InputCost
		record.OutputCost = settleResult.OutputCost
		record.CacheCreationCost = settleResult.CacheCreationCost
		record.CacheReadCost = settleResult.CacheReadCost
	}

	provider.RecordUsage(context.Background(), record)
}

// extractModelFromEvent 从 WebSocket 事件中提取模型名称
func extractModelFromEvent(event map[string]json.RawMessage) string {
	// 顶层 model 字段
	if m, ok := event["model"]; ok {
		var s string
		if json.Unmarshal(m, &s) == nil {
			return strings.Trim(s, `"`)
		}
	}

	// session.update 的 session.model
	if sessionRaw, ok := event["session"]; ok {
		var sess map[string]json.RawMessage
		if json.Unmarshal(sessionRaw, &sess) == nil {
			if m, ok := sess["model"]; ok {
				var s string
				if json.Unmarshal(m, &s) == nil {
					return strings.Trim(s, `"`)
				}
			}
		}
	}

	return ""
}
