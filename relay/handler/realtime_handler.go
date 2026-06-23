package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gorilla/websocket"

	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// websocketUpgrader HTTP → WebSocket 升级器
var websocketUpgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"},
	CheckOrigin:  func(r *http.Request) bool { return true },
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
}

// HandleRealtime 处理 /v1/realtime WebSocket 请求
func HandleRealtime(w http.ResponseWriter, r *http.Request, rc *RealtimeContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	ctx := r.Context()

	// 1. WebSocket 升级
	clientConn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		g.Log().Errorf(ctx, "[HandleRealtime] WebSocket upgrade failed: %v", err)
		return nil, nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}
	defer clientConn.Close()

	// 2. 读取第一条消息获取 model 名称
	_, firstMsg, err := clientConn.ReadMessage()
	if err != nil {
		return nil, nil, fmt.Errorf("read first websocket message failed: %w", err)
	}

	var firstEvent map[string]json.RawMessage
	if err := json.Unmarshal(firstMsg, &firstEvent); err != nil {
		return nil, nil, fmt.Errorf("parse first websocket message failed: %w", err)
	}

	modelName := extractModelFromEvent(firstEvent)
	if modelName == "" {
		errMsg, _ := json.Marshal(map[string]any{
			"type":  "error",
			"error": map[string]string{"message": "model is required in the first message"},
		})
		_ = clientConn.WriteMessage(websocket.TextMessage, errMsg)
		return nil, nil, fmt.Errorf("model not found in first websocket message")
	}

	billingResult := &BillingResult{}
	if billing != nil {
		if !billing.CheckScope(rc.Scope, "realtime") {
			return nil, billingResult, constant.NewAuthError("API key scope denied")
		}
		if !billing.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
			return nil, billingResult, constant.NewAuthError("IP address is not allowed")
		}
		allowed, limitLevel, limit, remaining, resetAt := billing.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, rc.KeyRateLimitQps)
		if !allowed {
			return nil, billingResult, &RelayErrorWithRateLimit{
				StatusCode: 429,
				Message:    fmt.Sprintf("rate limit exceeded at %s level", limitLevel),
				LimitLevel: limitLevel,
				Remaining:  remaining,
				ResetAt:    resetAt,
			}
		}
		billingResult.RateLimitInfo = &common.RateLimitInfo{
			Limit:     limit,
			Remaining: remaining,
			ResetAt:   resetAt,
		}
	}

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

	// 4. 渠道选择
	selection, err := provider.GetChannelForModel(ctx, rc.TenantID, rc.UserID, modelName, nil)
	if err != nil {
		return nil, nil, constant.NewChannelError("no available channel for model: "+modelName, err)
	}

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

		if err := billing.CheckMemberQuota(ctx, rc.TenantID, rc.UserID, 0); err != nil {
			return nil, billingResult, constant.NewQuotaError("member quota exceeded", err)
		}
		if err := billing.CheckApiKeyQuota(ctx, rc.ApiKeyID, 0); err != nil {
			return nil, billingResult, constant.NewQuotaError("API key quota exceeded", err)
		}

		amt, billErr := billing.PreDeduct(ctx, rc.TenantID, modelName, 0, 0, false, rc.RequestID)
		if billErr != nil {
			return nil, nil, constant.NewQuotaError("insufficient balance", billErr)
		}
		preDeductAmount = amt
		if err := billing.CheckApiKeyQuota(ctx, rc.ApiKeyID, amt); err != nil {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, amt)
			return nil, billingResult, constant.NewQuotaError("API key quota exceeded", err)
		}
	}

	billingResult.PreDeductAmount = preDeductAmount

	// 8. 建立 Realtime 代理
	proxy := openai.NewRealtimeProxy(info)
	if err := proxy.DialUpstream(); err != nil {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		return nil, billingResult, err
	}
	defer proxy.Close()

	// 把第一条消息转发到上游
	if err := proxy.GetTargetConn().WriteMessage(websocket.TextMessage, firstMsg); err != nil {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		return nil, billingResult, err
	}

	// 9. 启动双向代理
	usage, proxyErr := proxy.Proxy(ctx)

	// 10. 结算费用
	if billing != nil && preDeductAmount > 0 {
		if proxyErr != nil {
			streamUsage := usage
			if streamUsage == nil {
				streamUsage = &common.Usage{}
			}
			settleResult, _ := billing.SettleStreamInterrupted(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
				modelName, rc.RequestID, "realtime", streamUsage, preDeductAmount, rc.ProjectID)
			if settleResult != nil && settleResult.ActualCost > 0 {
				billing.IncrMemberQuotaUsed(ctx, rc.TenantID, rc.UserID, settleResult.ActualCost)
				billing.IncrApiKeyQuotaUsed(ctx, rc.ApiKeyID, settleResult.ActualCost)
			}
		} else if usage != nil {
			settleResult, _ := billing.Settle(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
				modelName, rc.RequestID, "realtime", usage, preDeductAmount, rc.ProjectID)
			if settleResult != nil && settleResult.ActualCost > 0 {
				billing.IncrMemberQuotaUsed(ctx, rc.TenantID, rc.UserID, settleResult.ActualCost)
				billing.IncrApiKeyQuotaUsed(ctx, rc.ApiKeyID, settleResult.ActualCost)
			}
		}
	}

	return usage, billingResult, proxyErr
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
