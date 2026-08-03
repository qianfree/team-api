package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dispatchadapter"
	commonlogic "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	tenantlogic "github.com/qianfree/team-api/internal/logic/tenant"
	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/override"
	"github.com/qianfree/team-api/relaykit/dispatch"
)

// RelayContext relay 请求上下文（从 GoFrame handler 传入）
type RelayContext struct {
	TenantID        int64
	UserID          int64
	ApiKeyID        int64
	ProjectID       int64 // 通过 API Key 关联的项目 ID
	RequestID       string
	Writer          http.ResponseWriter
	Scope           string                  // API Key scope
	ClientIP        string                  // 客户端 IP
	KeyRateLimitQps int                     // API Key QPS 覆盖值，0 表示使用全局默认
	KeyConcurrency  int                     // API Key 并发限制，0 表示不限制
	KeyIpWhitelist  string                  // API Key IP 白名单
	KeyTotalQuota   float64                 // API Key 总额度，0 表示不限制
	KeyUsedQuota    float64                 // API Key 已用额度（仅展示/审计，额度检查会鲜读 DB）
	ForwardingTrace *common.ForwardingTrace // 转发路径追踪（仅管理员可见）
}

// BillingResult 计费结果（返回给调用方用于设置响应头）
type BillingResult struct {
	PreDeductAmount float64
	ActualCost      float64
	RateLimitInfo   *common.RateLimitInfo
	Deprecation     *common.DeprecationInfo
	FirstTokenMs    int
}

// relayValidation 请求校验结果，供重试循环使用
type relayValidation struct {
	relayMode       constant.RelayMode
	relayModeStr    string
	modelName       string
	lookupModel     string
	thinkingInfo    *helper.ThinkingInfo
	isStream        bool
	estimatedTokens int
	maxTokens       int
	depInfo         *common.DeprecationInfo
	billingResult   *BillingResult
	sessionSignals  dispatch.SessionSignals // 请求体中的会话信号（影子模式/新调度用）
}

// validateRelayRequest 校验请求合法性：relay mode、QPS 限流（前置）、模型存在性、弃用状态、成员/API Key 模型范围。
// 纯校验逻辑，无 defer 副作用。
func validateRelayRequest(
	ctx context.Context,
	body []byte,
	path string,
	rc *RelayContext,
	provider common.DataProvider,
	billing common.BillingProvider,
) (*relayValidation, error) {
	// 1. 确定 relay mode
	relayMode := constant.Path2RelayMode(path)
	if relayMode == constant.RelayModeUnknown {
		g.Log().Errorf(ctx, "[RelayHandler] Unknown relay mode for path: %s", path)
		return nil, constant.NewRequestError("unsupported endpoint: "+path, nil)
	}

	// 1.5 QPS 限流检查（尽可能前置：只依赖认证阶段就已就绪的 tenant/user/key ID，
	// 提前到请求体解析和模型权限检查之前，让超限请求以 1 次 Redis EVAL 的成本被拒绝，
	// 不再为其解析大 JSON、查模型缓存。副作用：无效请求（坏 body/无权限模型）也计入
	// QPS 计数——这是期望行为，探测型流量同样应被限流兜住。
	billingResult := &BillingResult{}
	if billing != nil {
		allowed, limitLevel, limit, remaining, resetAt := billing.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, rc.KeyRateLimitQps)
		if !allowed {
			return nil, &RelayErrorWithRateLimit{
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

	relayModeStr := relayModeString(relayMode)
	if billing != nil {
		if !billing.CheckScope(rc.Scope, relayModeStr) {
			return nil, constant.NewAuthError("API key scope denied")
		}
		if !billing.CheckIPWhitelist(rc.KeyIpWhitelist, rc.ClientIP) {
			return nil, constant.NewAuthError("IP address is not allowed")
		}
	}

	// 2. 解析请求体获取模型名
	var rawRequest map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawRequest); err != nil {
		return nil, constant.NewRequestError("invalid request body", err)
	}

	modelName := ""
	if v, ok := rawRequest["model"]; ok {
		var m string
		if err := json.Unmarshal(v, &m); err == nil {
			modelName = m
		}
	}
	if modelName == "" {
		return nil, constant.NewRequestError("model is required", nil)
	}

	// 2.5 解析 thinking/effort 后缀
	parsed := helper.ParseThinkingSuffix(modelName)
	thinkingInfo := &parsed
	lookupModel := modelName
	if thinkingInfo.BaseModel != modelName {
		lookupModel = thinkingInfo.BaseModel
	}

	// 3. 验证模型存在且活跃
	_, _, modelErr := provider.GetModelMapping(ctx, modelName)
	if modelErr != nil && thinkingInfo.BaseModel != modelName {
		// 完整模型名查找失败，尝试基础模型名
		_, _, modelErr = provider.GetModelMapping(ctx, thinkingInfo.BaseModel)
		if modelErr == nil {
			lookupModel = thinkingInfo.BaseModel
		}
	}
	if modelErr != nil {
		if modelErr == common.ErrModelNotFound {
			return nil, constant.NewRequestError("model not found: "+modelName, modelErr)
		}
		return nil, modelErr
	}

	// 3.5 检查模型弃用状态
	depInfo, _ := provider.GetModelDeprecationInfo(ctx, lookupModel)
	if depInfo != nil && depInfo.Deprecated && depInfo.SunsetDate != "" {
		sunsetTime, _ := time.Parse("2006-01-02", depInfo.SunsetDate)
		if !sunsetTime.IsZero() && time.Now().After(sunsetTime) {
			return nil, constant.NewModelGoneError(lookupModel, depInfo.SunsetDate)
		}
	}
	billingResult.Deprecation = depInfo

	// 3.7 检查成员模型范围
	if allowed, err := provider.CheckMemberModelAccess(ctx, rc.TenantID, rc.UserID, lookupModel); err != nil {
		return nil, err
	} else if !allowed {
		return nil, constant.NewAuthError("model not allowed for this member")
	}

	// 4. API Key 模型范围校验
	if allowed, err := provider.CheckApiKeyModelAccess(ctx, rc.ApiKeyID, lookupModel); err != nil {
		return nil, err
	} else if !allowed {
		return nil, constant.NewAuthError("model not allowed for this API key")
	}

	// 5. QPS 限流已前置到步骤 1.5（见上），此处不再检查

	// 7. 估算输入 token 数
	estimatedInputTokens := estimateInputTokens(body)

	var isStream bool
	if streamVal, ok := rawRequest["stream"]; ok {
		_ = json.Unmarshal(streamVal, &isStream)
	}

	var maxTokens int
	if mtVal, ok := rawRequest["max_tokens"]; ok {
		_ = json.Unmarshal(mtVal, &maxTokens)
	}
	if mcVal, ok := rawRequest["max_completion_tokens"]; ok {
		var mc int
		_ = json.Unmarshal(mcVal, &mc)
		if mc > maxTokens {
			maxTokens = mc
		}
	}

	return &relayValidation{
		relayMode:       relayMode,
		relayModeStr:    relayModeStr,
		modelName:       modelName,
		lookupModel:     lookupModel,
		thinkingInfo:    thinkingInfo,
		isStream:        isStream,
		estimatedTokens: estimatedInputTokens,
		maxTokens:       maxTokens,
		depInfo:         depInfo,
		billingResult:   billingResult,
		sessionSignals:  extractSessionSignals(rawRequest),
	}, nil
}

// extractSessionSignals 从已解析的请求体提取会话信号（解析链的协议内信号部分）。
// 复用 rawRequest 的一次解析，不额外反序列化整个 body。
func extractSessionSignals(rawRequest map[string]json.RawMessage) dispatch.SessionSignals {
	var sig dispatch.SessionSignals
	if v, ok := rawRequest["metadata"]; ok {
		var meta struct {
			UserID string `json:"user_id"`
		}
		if json.Unmarshal(v, &meta) == nil {
			sig.AnthropicUserID = meta.UserID
		}
	}
	if v, ok := rawRequest["previous_response_id"]; ok {
		_ = json.Unmarshal(v, &sig.PreviousResponseID)
	}
	if v, ok := rawRequest["conversation_id"]; ok {
		_ = json.Unmarshal(v, &sig.ConversationID)
	}
	return sig
}

// settleSuccessfulRequest 成功路径的计费结算、健康度更新和用量记录。
func settleSuccessfulRequest(
	rc *RelayContext,
	v *relayValidation,
	usage *common.Usage,
	info *common.RelayInfo,
	selection *common.ChannelSelection,
	preDeductAmount float64,
	provider common.DataProvider,
	billing common.BillingProvider,
	headers http.Header,
	path string,
) *BillingResult {
	// 16. 结算费用（使用完整 Usage 含 cache token）
	// 重新创建 context，上游 DoResponse 可能耗时很长（长文本流式输出）
	postCtx, postCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer postCancel()

	var settleResult *common.SettlementResult
	if billing != nil && preDeductAmount > 0 {
		settleResult = billing.SettleWithUsage(postCtx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
			v.modelName, rc.RequestID, v.relayModeStr,
			usage, preDeductAmount, info)
		if settleResult != nil {
			v.billingResult.ActualCost = settleResult.ActualCost
		} else {
			g.Log().Warningf(postCtx, "[RelayHandler] Settlement failed for request=%s model=%s, refunding pre-deduct amount=%.6f",
				rc.RequestID, v.modelName, preDeductAmount)
			_ = billing.SettleFailed(postCtx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
	}

	// 16.5 累加成员已用额度（幂等重复结算 DuplicateSkip 时不得重复累加）
	if billing != nil && settleResult != nil && settleResult.ActualCost > 0 && !settleResult.DuplicateSkip {
		billing.IncrMemberQuotaUsed(postCtx, rc.TenantID, rc.UserID, settleResult.ActualCost)
		billing.IncrApiKeyQuotaUsed(postCtx, rc.ApiKeyID, settleResult.ActualCost)
	}
	if settleResult != nil && settleResult.ActualCost > 0 && rc.ProjectID > 0 {
		if err := tenantlogic.CheckProjectBudget(postCtx, rc.TenantID, rc.ProjectID); err != nil {
			g.Log().Warningf(postCtx, "[RelayHandler] Check project budget failed: tenant=%d project=%d request=%s err=%v",
				rc.TenantID, rc.ProjectID, rc.RequestID, err)
		}
	}

	// 17. 记录用量（健康上报与绑定续期已由调度会话 Finish 完成）
	firstTokenMs := 0
	if !info.FirstResponseTime.IsZero() {
		firstTokenMs = int(info.FirstResponseTime.Sub(info.StartTime).Milliseconds())
	}

	// 构建用量记录
	usageRecord := &common.UsageRecord{
		TenantID:         rc.TenantID,
		UserID:           rc.UserID,
		ApiKeyID:         rc.ApiKeyID,
		ProjectID:        rc.ProjectID,
		ChannelID:        selection.ChannelID,
		ModelName:        v.modelName,
		RelayMode:        int(v.relayMode),
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CachedTokens:     tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedTokens }),
		AudioTokens: tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.AudioTokens }) +
			tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.AudioTokens }),
		ImageTokens:     tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.ImageTokens }),
		ReasoningTokens: tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.ReasoningTokens }),
		LatencyMs:       info.TotalLatencyMs(),
		IsStream:        v.isStream,
		Success:         true,
		RequestID:       rc.RequestID,
		Status:          "success",

		// Cache token 明细
		CacheCreationTokens:   usage.CacheCreationTokens,
		CacheCreation5mTokens: tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedCreation5mTokens }),
		CacheCreation1hTokens: tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedCreation1hTokens }),
		CacheReadTokens:       tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedTokens }),

		// 音频 token 分离
		AudioInputTokens:  tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.AudioTokens }),
		AudioOutputTokens: tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.AudioTokens }),

		// 其他 token
		ImageOutputTokens: tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.ImageTokens }),

		// 请求元数据
		RequestedModel:  v.modelName,
		UpstreamModel:   selection.UpstreamModelName,
		RequestType:     requestType(v.isStream),
		UserAgent:       headers.Get("User-Agent"),
		ClientIP:        rc.ClientIP,
		FirstTokenMs:    firstTokenMs,
		ReasoningEffort: info.ReasoningEffort,
		InboundEndpoint: path,

		// 渠道详情
		ChannelName: selection.ChannelName,
		ChannelType: selection.ChannelType,

		// 重试
		RetryIndex: info.RetryIndex,
	}

	// 填充结算费用数据
	if settleResult != nil {
		usageRecord.TotalCost = settleResult.BaseCost
		usageRecord.ActualCost = settleResult.ActualCost
		usageRecord.Currency = "USD"
		usageRecord.PreDeductAmount = settleResult.PreDeductAmount
		usageRecord.RefundAmount = settleResult.RefundAmount
		usageRecord.SupplementAmount = settleResult.SupplementAmount
		usageRecord.BillingSnapshot = settleResult.BillingSnapshot
		usageRecord.BillingSummary = settleResult.BillingSummary
		usageRecord.BillingMode = settleResult.BillingMode
		usageRecord.BillingSource = settleResult.BillingSource
		usageRecord.RateMultiplier = settleResult.RateMultiplier
		usageRecord.InputCost = settleResult.InputCost
		usageRecord.OutputCost = settleResult.OutputCost
		usageRecord.CacheCreationCost = settleResult.CacheCreationCost
		usageRecord.CacheReadCost = settleResult.CacheReadCost
	}

	v.billingResult.FirstTokenMs = firstTokenMs
	provider.RecordUsage(postCtx, usageRecord)

	return v.billingResult
}

// RelayHandler 共享的 relay 请求编排逻辑（带重试 + 计费）
func RelayHandler(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	// 客户端已断开的请求直接终止（fail-fast）：断开风暴下若继续空转流水线，
	// 额度检查会误报 "load failed ... skipping check"、调度器会把 canceled ctx
	// 误报成"无可用渠道"，还平白消耗限流/并发/DB 资源。
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	// 校验请求（relay mode、模型、权限、限流）
	v, err := validateRelayRequest(ctx, body, path, rc, provider, billing)
	if err != nil {
		return nil, nil, err
	}

	// 在写入任何响应体之前设置限流/弃用 header（流式响应中 WriteHeader 后无法追加）
	setPreResponseHeaders(rc.Writer, v.billingResult)

	// 资源准备（并发控制 + 监控注册 + 预扣）
	if billing != nil {
		if !billing.AcquireConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, v.modelName) {
			return nil, nil, constant.NewRateLimitError("concurrent request limit exceeded")
		}
		defer billing.ReleaseConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, v.modelName)

		if !billing.AcquireApiKeyConcurrent(ctx, rc.ApiKeyID, rc.KeyConcurrency) {
			return nil, nil, constant.NewRateLimitError("API key concurrent request limit exceeded")
		}
		defer billing.ReleaseApiKeyConcurrent(ctx, rc.ApiKeyID)
	}

	monitor.RegisterRequest(&monitor.TrackedRequest{
		RequestID: rc.RequestID,
		TenantID:  rc.TenantID,
		UserID:    rc.UserID,
		ProjectID: rc.ProjectID,
		ModelName: v.modelName,
		IsStream:  v.isStream,
		StartTime: time.Now(),
		Path:      path,
	})
	defer monitor.UnregisterRequest(rc.RequestID)

	var preDeductAmount float64
	if billing != nil {
		amt, err := billing.PreDeduct(ctx, rc.TenantID, v.modelName, v.estimatedTokens, v.maxTokens, v.isStream, rc.RequestID)
		if err != nil {
			// 未配价模型 fail-closed：返回明确的请求错误，而非误导性的"余额不足"
			if errors.Is(err, common.ErrModelPricingNotConfigured) {
				return nil, nil, constant.NewRequestError("model pricing not configured: "+v.modelName, err)
			}
			return nil, nil, constant.NewQuotaError("insufficient balance", err)
		}
		preDeductAmount = amt
		v.billingResult.PreDeductAmount = amt
		// 成员/Key 额度检查只做这一次（带实际预扣额）：额度是"控制线"计数器而非资源冻结，
		// 预扣前后检查的竞态语义相同。原先预扣前额外的 Check(0) 快速闸门对全量请求
		// 各多付一次 Redis 往返，只为让"额度已耗尽"的请求少走一步预扣+退款，得不偿失
		//（该场景已被上游 QPS 限流兜底），故移除。
		if err := billing.CheckApiKeyQuota(ctx, rc.ApiKeyID, amt); err != nil {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, amt)
			return nil, nil, constant.NewQuotaError("API key quota exceeded", err)
		}
		if err := billing.CheckMemberQuota(ctx, rc.TenantID, rc.UserID, amt); err != nil {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, amt)
			return nil, nil, constant.NewQuotaError("member quota exceeded", err)
		}
	}

	// 渠道调度与请求执行（新调度引擎：Route / Next / Report / Finish）
	// 重试语义由调度核心的 FSM 决定（三预算：原地/凭证轮换/failover），handler 只执行决策。
	channelErrors := make([]string, 0)

	trace := &common.ForwardingTrace{
		EntryPath:      path,
		EntryFormat:    string(relayModeToInboundFormat(v.relayMode)),
		RequestedModel: v.modelName,
	}

	// 租户模型权限与渠道范围校验（原 GetChannelForModel 的第一步，不属于调度模块）
	modelEnabled, channelScope, err := provider.CheckTenantModelAccess(ctx, rc.TenantID, v.lookupModel)
	if err != nil {
		// 客户端已断开：权限查询被中断不是渠道/权限问题，静默退款退出，
		// 不记失败用量、不打无可用渠道告警（与下方 MaterializeSelection 的处理一致）
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(context.WithoutCancel(ctx), rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, v.billingResult, err
		}
		result := handleChannelUnavailable(ctx, billing, provider, rc, v, preDeductAmount, channelErrors, err, "")
		return result.usage, result.billingResult, result.err
	}
	if !modelEnabled {
		result := handleChannelUnavailable(ctx, billing, provider, rc, v, preDeductAmount, channelErrors, common.ErrTenantModelNotEnabled, "")
		return result.usage, result.billingResult, result.err
	}

	sig := v.sessionSignals
	sig.HeaderSessionID = headers.Get("X-Session-Id")
	co := dispatchadapter.Coordinator(ctx)
	sess := co.Route(ctx, dispatch.RequestProfile{
		RequestID: rc.RequestID,
		TenantID:  rc.TenantID,
		UserID:    rc.UserID,
		APIKeyID:  rc.ApiKeyID,
		Model:     v.lookupModel,
		Scope:     channelScope,
		Replay:    co.Policy().Replay.ReplayabilityForMode(v.relayModeStr),
		Signals:   sig,
		Policy:    dispatchadapter.TenantRoutingPolicy(ctx, rc.TenantID),
	})
	// 兜底：任何未显式 Finish 的退出路径释放租约（Finish 幂等，成功路径的显式调用优先生效）
	defer sess.Finish(context.WithoutCancel(ctx), false, 0)

	for attempt := 0; ; attempt++ {
		d := sess.Next(ctx)
		if d == nil {
			// 客户端已断开：调度器内部的 Redis 操作（快照/租约/绑定）被 canceled ctx
			// 中断会返回空决策，这不是"无可用渠道"——静默退款退出，不记失败用量、
			// 不打无可用渠道告警、不污染渠道健康统计
			if ctxErr := ctx.Err(); ctxErr != nil {
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(context.WithoutCancel(ctx), rc.TenantID, rc.RequestID, preDeductAmount)
				}
				return nil, v.billingResult, ctxErr
			}
			// 无可用渠道：附带调度器的排除原因摘要（各原因独立计数，仅列非零项）。
			// 例：熔断OPEN×1 / 容量租约满×1（并发超 softLimit） / 半开探测限流×1（恢复期每窗口仅 1 个探测）
			result := handleChannelUnavailable(ctx, billing, provider, rc, v, preDeductAmount, channelErrors, common.ErrChannelUnavailable, sess.NoChannelDiagnosis().Summary())
			return result.usage, result.billingResult, result.err
		}
		appendSchedulerDecision(trace, d, attempt)

		selection, mErr := provider.MaterializeSelection(ctx, d.Channel.ID, d.KeyID, v.lookupModel)
		if mErr != nil {
			// 客户端已断开（context.Canceled / DeadlineExceeded）：Key 查询被中断不代表渠道有问题，
			// 静默退出，不记入 channelErrors，不触发渠道健康惩罚，退还预扣款项后返回
			if errors.Is(mErr, context.Canceled) || errors.Is(mErr, context.DeadlineExceeded) {
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(context.WithoutCancel(ctx), rc.TenantID, rc.RequestID, preDeductAmount)
				}
				return nil, v.billingResult, mErr
			}
			// Key 解密失败 / 目录元数据缺失：按渠道级致命上报换渠道，不发起上游请求
			channelErrors = append(channelErrors, fmt.Sprintf("attempt=%d channel=%d materialize_error=[%v]", attempt, d.Channel.ID, mErr))
			if decision := reportMaterializeFailure(ctx, sess, mErr); decision == dispatch.DecisionAbort {
				result := handleChannelUnavailable(ctx, billing, provider, rc, v, preDeductAmount, channelErrors, common.ErrChannelUnavailable, "")
				return result.usage, result.billingResult, result.err
			}
			continue
		}
		selection.SelectionReason = string(d.Reason)
		selection.Weight = int(d.Channel.BaseWeight)
		selection.HealthScore = d.Breakdown.Health * 100

		// 阿里云百炼（DashScope）异步图片模型（wanx*、qwen-image-plus 等）上游为异步任务式
		// （提交拿 task_id → 轮询取图），其 image-synthesis 端点强制异步，无法经同步
		// /v1/images/generations 一次性返回图片。命中时直接引导客户端改用异步端点。
		// 判定收敛到 constant.IsAsyncImageModel（按 provider + 模型），qwen-image-2.x 等
		// 同步 multimodal 模型不在此列，会继续走下方同步转发。租户模型列表 async_image 标记同源。
		if v.relayMode == constant.RelayModeImagesGenerations &&
			constant.IsAsyncImageModel(constant.ProviderType(selection.ChannelType), selection.UpstreamModelName) {
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, v.billingResult, constant.NewRequestError(
				"this image model uses asynchronous generation on Alibaba DashScope; submit via POST /v1/images/generations/async and poll for the result", nil)
		}

		info := buildRelayInfo(ctx, rc, v, selection, path, headers, attempt)

		if tr := monitor.GetTrackedRequest(rc.RequestID); tr != nil {
			tr.ChannelID = selection.ChannelID
			tr.ChannelName = selection.ChannelName
		}

		adaptor := channel.GetAdaptor(selection.ChannelType)
		if adaptor == nil {
			g.Log().Errorf(ctx, "[RelayHandler] No adaptor found for channelType: %d", selection.ChannelType)
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, v.billingResult, fmt.Errorf("unsupported channel type: %d", selection.ChannelType)
		}
		adaptor.Init(info)

		hop := buildTraceHop(attempt, selection, adaptor, info)

		if info.ClientFormat == "" {
			info.ClientFormat = info.InboundFormat
		}

		// 转换请求（直连模式跳过协议转换和参数改写）
		convertedBody, err := convertRequestBody(ctx, info, body, adaptor)
		if err != nil {
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, v.billingResult, err
		}

		// 容量租约已由调度器在 Next 内获取；长流式请求由续期器保活
		leaseRefresh := startDispatchLeaseRefresher(d.Channel.ID, rc.RequestID)

		upstreamCtx := context.WithoutCancel(ctx)
		settleCtx, settleCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer settleCancel()

		// 发送请求到上游
		resp, err := adaptor.DoRequest(upstreamCtx, info, convertedBody)
		if err != nil {
			leaseRefresh.Stop()
			// 送达状态标注：连接拒绝/DNS/TLS 建连失败 = 确定未送达；
			// 写出后 EOF/RST/读超时 = 可能已送达。ReplayUnsafe+MaybeSent 由 FSM 硬规则终止。
			delivery := deliveryStateOfRequestErr(err)
			decision, backoff := sess.Report(settleCtx, dispatchStatusCode(err), err, delivery, info.LatencyMs(), 0)
			trackRetryDecision(dispatchStatusCode(err), err, delivery, decision)

			failReason := fmt.Sprintf("attempt=%d channel=%d(%s) model=%s upstreamModel=%s error=[%v] latency=%.0fms decision=%s",
				attempt, selection.ChannelID, selection.ChannelName, v.modelName, selection.UpstreamModelName, err, info.LatencyMs(), decision)
			channelErrors = append(channelErrors, failReason)
			g.Log().Warningf(ctx, "[RelayHandler] Upstream request failed: %s", failReason)

			if decision != dispatch.DecisionAbort {
				recordChannelError(rc, selection, v.modelName, attempt, false, err, info.LatencyMs())
				appendHop(trace, hop, false, err.Error(), info.LatencyMs())
				settleCancel()
				sleepBackoff(ctx, backoff)
				continue
			}

			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(settleCtx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			recordFailedUsage(provider, rc, selection.ChannelID, v.modelName, v.relayMode, v.isStream, err)
			recordChannelError(rc, selection, v.modelName, attempt, true, err, info.LatencyMs())
			finalizeTrace(trace, rc, hop, false, attempt, selection, err.Error(), info.LatencyMs())
			return nil, v.billingResult, helper.RemapStatusCode(constant.NewUpstreamError(502, "upstream request failed", err), info.ChannelMeta.Settings.StatusCodeMapping)
		}

		if !v.isStream {
			info.SetFirstResponseTime()
		}

		// 处理上游响应
		usage, err := adaptor.DoResponse(ctx, resp, info, rc.Writer)
		leaseRefresh.Stop()
		if err != nil {
			err = helper.RemapStatusCode(err, info.ChannelMeta.Settings.StatusCodeMapping)

			if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
				g.Log().Warningf(ctx, "[RelayHandler] Stream interrupted: adaptor=%s, model=%s, reason=%s",
					adaptor.GetChannelName(), v.modelName, info.StreamStatus.Summary())
				// 客户端已收到部分流：不可重试（响应已污染），上报健康后按流中断结算
				_, _ = sess.Report(settleCtx, dispatchStatusCode(err), err, dispatch.DeliveryResponseStarted, info.LatencyMs(), 0)
				streamUsage := usage
				if streamUsage == nil {
					streamUsage = &common.Usage{}
				}
				if billing != nil && preDeductAmount > 0 {
					settleResult, err := billing.SettleStreamInterrupted(settleCtx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
						v.modelName, rc.RequestID, v.relayModeStr, streamUsage, preDeductAmount, rc.ProjectID)
					if err != nil {
						g.Log().Warningf(settleCtx, "[RelayHandler] Stream interrupted settlement failed: tenant=%d project=%d request=%s err=%v",
							rc.TenantID, rc.ProjectID, rc.RequestID, err)
					} else if rc.ProjectID > 0 {
						if err := tenantlogic.CheckProjectBudget(settleCtx, rc.TenantID, rc.ProjectID); err != nil {
							g.Log().Warningf(settleCtx, "[RelayHandler] Check project budget failed: tenant=%d project=%d request=%s err=%v",
								rc.TenantID, rc.ProjectID, rc.RequestID, err)
						}
					}
					if settleResult != nil && settleResult.ActualCost > 0 && !settleResult.DuplicateSkip {
						billing.IncrMemberQuotaUsed(settleCtx, rc.TenantID, rc.UserID, settleResult.ActualCost)
						billing.IncrApiKeyQuotaUsed(settleCtx, rc.ApiKeyID, settleResult.ActualCost)
					}
				}
				recordFailedUsage(provider, rc, selection.ChannelID, v.modelName, v.relayMode, v.isStream, err)
				finalizeTrace(trace, rc, hop, false, attempt, selection, err.Error(), info.LatencyMs())
				return usage, v.billingResult, err
			}

			// 送达状态判定：
			// 1. adaptor 显式标记 ResponseWritten → 已写出，必须终止；
			// 2. 流式请求且流已实际开始（StreamStatus 已有结束原因）→ 已写出 SSE 数据，必须终止；
			// 3. 流式请求但流尚未开始（上游在 SSE 头之前返回错误，StreamStatus 无结束原因）
			//    → 未向客户端写出任何字节，允许换渠道重试；
			// 4. 非流式请求 → 已收到完整错误响应，可按分类决定原地重试/换渠道。
			delivery := dispatch.DeliveryResponseReceived
			if constant.IsResponseWritten(err) {
				delivery = dispatch.DeliveryResponseStarted
			} else if v.isStream && (info.StreamStatus == nil || info.StreamStatus.GetEndReason() != "") {
				// StreamStatus 为 nil：保守兜底，按旧行为终止（遗留流式处理器可能未初始化 StreamStatus）
				// GetEndReason() != ""：流已开始传输后出错，字节已提交给客户端，必须终止
				delivery = dispatch.DeliveryResponseStarted
			}
			decision, backoff := sess.Report(settleCtx, dispatchStatusCode(err), err, delivery, info.LatencyMs(), retryAfterOf(err))
			trackRetryDecision(dispatchStatusCode(err), err, delivery, decision)

			if decision != dispatch.DecisionAbort {
				g.Log().Warningf(ctx, "[RelayHandler] DoResponse failed (will retry): adaptor=%s, inboundFormat=%s, channel=%d(%s) model=%s attempt=%d error=%v latency=%.0fms decision=%s",
					adaptor.GetChannelName(), info.InboundFormat, selection.ChannelID, selection.ChannelName, v.modelName, attempt, err, info.LatencyMs(), decision)
			} else {
				// 上游响应错误属于预期内的运营事件（4xx/5xx/超时等），非代码 bug：
				// 用 Warningf 避免触发 glog 默认对 ERROR+ 级别自动打印调用栈（StStatus=1），
				// 堆栈只会复述对所有请求都一样的中间件链，污染日志且无诊断价值。
				g.Log().Warningf(ctx, "[RelayHandler] DoResponse failed (abort): adaptor=%s, inboundFormat=%s, channel=%d(%s) model=%s attempt=%d error=%v latency=%.0fms decision=%s",
					adaptor.GetChannelName(), info.InboundFormat, selection.ChannelID, selection.ChannelName, v.modelName, attempt, err, info.LatencyMs(), decision)
			}
			channelErrors = append(channelErrors, fmt.Sprintf("attempt=%d channel=%d(%s) model=%s doResponse_error=[%v] latency=%.0fms",
				attempt, selection.ChannelID, selection.ChannelName, v.modelName, err, info.LatencyMs()))

			if decision != dispatch.DecisionAbort {
				recordChannelError(rc, selection, v.modelName, attempt, false, err, info.LatencyMs())
				appendHop(trace, hop, false, err.Error(), info.LatencyMs())
				settleCancel()
				sleepBackoff(ctx, backoff)
				continue
			}

			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(settleCtx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			recordFailedUsage(provider, rc, selection.ChannelID, v.modelName, v.relayMode, v.isStream, err)
			recordChannelError(rc, selection, v.modelName, attempt, true, err, info.LatencyMs())
			finalizeTrace(trace, rc, hop, false, attempt, selection, err.Error(), info.LatencyMs())
			return nil, v.billingResult, err
		}

		// 成功路径：绑定续期 + 健康上报由 Finish 完成
		sess.Finish(settleCtx, true, info.LatencyMs())
		appendHop(trace, hop, true, "", info.LatencyMs())
		trace.TotalAttempts = attempt + 1
		trace.UpstreamModel = selection.UpstreamModelName
		trace.ModelMapped = selection.IsModelMapped
		rc.ForwardingTrace = trace

		result := settleSuccessfulRequest(rc, v, usage, info, selection, preDeductAmount, provider, billing, headers, path)
		return usage, result, nil
	}
}

// buildRelayInfo 从渠道选择结果构建 RelayInfo
// attempt 为当前重试轮次（0=首次），写入 RetryIndex 供 ParamOverride「是否重试」规则与 bil_usage_logs.retry_index 使用（C3）。
func buildRelayInfo(ctx context.Context, rc *RelayContext, v *relayValidation, selection *common.ChannelSelection, path string, headers http.Header, attempt int) *common.RelayInfo {
	info := &common.RelayInfo{
		Context:          ctx,
		TenantID:         rc.TenantID,
		UserID:           rc.UserID,
		ApiKeyID:         rc.ApiKeyID,
		ProjectID:        rc.ProjectID,
		RequestID:        rc.RequestID,
		RetryIndex:       attempt,
		RelayMode:        int(v.relayMode),
		IsStream:         v.isStream,
		OriginModelName:  v.modelName,
		BaseModelName:    v.lookupModel,
		ThinkingEnabled:  v.thinkingInfo.IsThinking,
		ThinkingDisabled: v.thinkingInfo.IsNoThinking,
		ReasoningEffort:  v.thinkingInfo.EffortLevel,
		RequestURLPath:   path,
		RequestHeaders:   headers,
		StartTime:        time.Now(),
		StreamStatus:     common.NewStreamStatus(),
		InboundFormat:    relayModeToInboundFormat(v.relayMode),
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
	}
	// 流中断结算时上游 usage 常缺失，记录请求侧输入估算值（与预扣同源）供输入计费兜底
	info.SetEstimatePromptTokens(v.estimatedTokens)
	return info
}

// convertRequestBody 根据是否直连模式转换请求体
func convertRequestBody(ctx context.Context, info *common.RelayInfo, body []byte, adaptor common.Adaptor) (io.Reader, error) {
	if canPassThrough(info) {
		if info.InboundFormat == constant.RelayFormatGemini {
			body = helper.StripStreamField(body)
		}
		return bytes.NewReader(body), nil
	}

	// relaykit 转换器路径（特性开关控制，默认关闭）。失败/未启用回退旧代码路径。
	var convertedBody io.Reader
	if relaykitBody, ok := tryConvertRequestViaRelaykit(ctx, info, body); ok {
		convertedBody = relaykitBody
	} else {
		legacyBody, err := adaptor.ConvertRequest(ctx, info, body)
		if err != nil {
			g.Log().Errorf(ctx, "[RelayHandler] ConvertRequest failed: adaptor=%s, inboundFormat=%s, error=%v",
				adaptor.GetChannelName(), info.InboundFormat, err)
			return nil, err
		}
		convertedBody = legacyBody
	}

	// 注入渠道系统提示词
	if info.ChannelMeta.Settings.SystemPrompt != "" {
		bodyBytes, _ := io.ReadAll(convertedBody)
		bodyBytes = helper.InjectSystemPrompt(bodyBytes, info)
		convertedBody = bytes.NewReader(bodyBytes)
	}

	// 应用请求体改写（ParamOverride）
	if info.ChannelMeta.Settings.ParamOverride != nil {
		bodyBytes, err := io.ReadAll(convertedBody)
		if err != nil {
			g.Log().Errorf(ctx, "[RelayHandler] Read converted body for param override failed: %v", err)
			return nil, err
		}
		bodyBytes, err = override.ApplyParamOverride(bodyBytes, info)
		if err != nil {
			if retErr, ok := override.AsReturnError(err); ok {
				return nil, constant.NewUpstreamError(retErr.StatusCode, retErr.Message, retErr)
			}
			return nil, err
		}
		convertedBody = bytes.NewReader(bodyBytes)
	}

	// 字段清理
	sanitized, _ := io.ReadAll(convertedBody)
	sanitized = helper.SanitizeFields(sanitized, info.ChannelMeta.Settings)
	return bytes.NewReader(sanitized), nil
}

// channelUnavailableResult 渠道不可用时的返回值
type channelUnavailableResult struct {
	usage         *common.Usage
	billingResult *BillingResult
	err           error
}

// handleChannelUnavailable 处理渠道选择失败：退还预扣、记录错误
func handleChannelUnavailable(
	ctx context.Context,
	billing common.BillingProvider,
	provider common.DataProvider,
	rc *RelayContext,
	v *relayValidation,
	preDeductAmount float64,
	channelErrors []string,
	err error,
	noChannelDiag string, // 调度器无可用渠道时的排除原因摘要（NoChannelDiag.Summary()），无诊断信息传 ""
) *channelUnavailableResult {
	if err != common.ErrChannelUnavailable {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		return &channelUnavailableResult{nil, v.billingResult, err}
	}

	diagSuffix := ""
	if noChannelDiag != "" {
		diagSuffix = " 原因: " + noChannelDiag
	}

	if len(channelErrors) > 0 {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		// 全部渠道失败是真实运营告警，保留 ERROR 级别；但调用栈固定无意义，禁用堆栈打印
		g.Log().Stack(false).Errorf(ctx, "[RelayHandler] All %d channels failed for model=%s tenant=%d user=%d request=%s.%s Failure details: %s",
			len(channelErrors), v.modelName, rc.TenantID, rc.UserID, rc.RequestID, diagSuffix, strings.Join(channelErrors, "\n"))
		allFailedErr := constant.NewChannelError(
			fmt.Sprintf("all %d channels failed for model: %s", len(channelErrors), v.modelName),
			constant.ErrAllChannelsFailed,
		)
		recordFailedUsage(provider, rc, 0, v.modelName, v.relayMode, v.isStream, allFailedErr)
		return &channelUnavailableResult{nil, v.billingResult, allFailedErr}
	}

	if billing != nil && preDeductAmount > 0 {
		_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
	}
	// 无可用渠道属于正常业务条件（模型未配渠道/容量满/熔断中等），降级为 Warning 并禁用堆栈打印；
	// 具体原因由调度器的排除明细摘要给出（熔断OPEN/半开探测限流/容量租约满/凭证冷却/目录为空）
	g.Log().Stack(false).Warningf(ctx, "[RelayHandler] 无可用渠道: model=%s tenant=%d user=%d request=%s%s",
		v.modelName, rc.TenantID, rc.UserID, rc.RequestID, diagSuffix)
	noChErr := constant.NewChannelError("no available channel for model: "+v.modelName, err)
	recordFailedUsage(provider, rc, 0, v.modelName, v.relayMode, v.isStream, noChErr)
	return &channelUnavailableResult{nil, v.billingResult, noChErr}
}

// buildTraceHop 构建转发追踪的单跳记录
func buildTraceHop(attempt int, selection *common.ChannelSelection, adaptor common.Adaptor, info *common.RelayInfo) common.ForwardingHop {
	var upstreamURL string
	if u, err := adaptor.GetRequestURL(info); err == nil {
		upstreamURL = u
	}
	return common.ForwardingHop{
		Attempt:         attempt,
		ChannelID:       selection.ChannelID,
		ChannelName:     selection.ChannelName,
		ChannelType:     selection.ChannelType,
		Provider:        constant.ProviderType(selection.ChannelType).String(),
		BaseURL:         selection.BaseURL,
		UpstreamURL:     upstreamURL,
		UpstreamModel:   selection.UpstreamModelName,
		ModelMapped:     selection.IsModelMapped,
		SelectionReason: selection.SelectionReason,
		Priority:        selection.Priority,
		Weight:          selection.Weight,
		HealthScore:     selection.HealthScore,
	}
}

// appendHop 追加一条 hop 到 trace
func appendHop(trace *common.ForwardingTrace, hop common.ForwardingHop, success bool, errMsg string, latencyMs float64) {
	hop.Success = success
	hop.Error = errMsg
	hop.LatencyMs = latencyMs
	trace.Hops = append(trace.Hops, hop)
}

// finalizeTrace 设置 trace 的最终状态并写入 rc
func finalizeTrace(trace *common.ForwardingTrace, rc *RelayContext, hop common.ForwardingHop, success bool, attempt int, selection *common.ChannelSelection, errMsg string, latencyMs float64) {
	hop.Success = success
	hop.Error = errMsg
	hop.LatencyMs = latencyMs
	trace.Hops = append(trace.Hops, hop)
	trace.TotalAttempts = attempt + 1
	trace.UpstreamModel = selection.UpstreamModelName
	trace.ModelMapped = selection.IsModelMapped
	rc.ForwardingTrace = trace
}

// RelayErrorWithRateLimit 带 429 限流信息的错误
type RelayErrorWithRateLimit struct {
	StatusCode int
	Message    string
	LimitLevel string
	Remaining  int
	ResetAt    int64
}

func (e *RelayErrorWithRateLimit) Error() string {
	return e.Message
}

// recordFailedUsage 记录失败用量
func recordFailedUsage(provider common.DataProvider, rc *RelayContext, channelID int64, modelName string, relayMode constant.RelayMode, isStream bool, err error) {
	provider.RecordUsage(context.Background(), &common.UsageRecord{
		TenantID:     rc.TenantID,
		UserID:       rc.UserID,
		ApiKeyID:     rc.ApiKeyID,
		ProjectID:    rc.ProjectID,
		ChannelID:    channelID,
		ModelName:    modelName,
		RelayMode:    int(relayMode),
		LatencyMs:    0,
		IsStream:     isStream,
		Success:      false,
		RequestID:    rc.RequestID,
		Status:       "error",
		ErrorMessage: err.Error(),
	})
}

// recordChannelError 记录渠道错误事件到 chn_error_events（异步，不阻塞请求）
func recordChannelError(rc *RelayContext, selection *common.ChannelSelection, modelName string, attempt int, isFinal bool, err error, latencyMs float64) {
	if commonlogic.DefaultChannelErrorWriter == nil {
		return
	}

	errMsg := err.Error()
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}

	statusCode := 0
	errType := "unknown"
	var relayErr *constant.RelayError
	if errors.As(err, &relayErr) {
		statusCode = relayErr.StatusCode
		errType = relayErr.Type
	}

	event := map[string]any{
		"channel_id":     selection.ChannelID,
		"channel_name":   selection.ChannelName,
		"channel_type":   selection.ChannelType,
		"provider":       constant.ProviderType(selection.ChannelType).String(),
		"model_name":     modelName,
		"request_id":     rc.RequestID,
		"tenant_id":      rc.TenantID,
		"error_category": constant.ClassifyError(err),
		"status_code":    statusCode,
		"error_type":     errType,
		"error_message":  errMsg,
		"is_retryable":   constant.IsRetryable(err),
		"attempt":        attempt,
		"is_final":       isFinal,
		"latency_ms":     latencyMs,
	}
	if selection.UpstreamModelName != "" {
		event["upstream_model"] = selection.UpstreamModelName
	}
	commonlogic.DefaultChannelErrorWriter.Submit(event)
}

// estimateInputTokens 粗略估算输入 token 数（按字符数 / 4）
func estimateInputTokens(body []byte) int {
	return len(body) / 4
}

// tokenDetailField 安全提取 TokenDetails 中的字段值
func tokenDetailField(details *common.TokenDetails, getter func(*common.TokenDetails) int) int {
	if details == nil {
		return 0
	}
	return getter(details)
}

// setPreResponseHeaders 在写入响应体之前设置限流和弃用 header
func setPreResponseHeaders(w http.ResponseWriter, br *BillingResult) {
	if br == nil {
		return
	}
	if info := br.RateLimitInfo; info != nil {
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", info.Limit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", info.Remaining))
		w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", info.ResetAt))
	}
	if dep := br.Deprecation; dep != nil {
		w.Header().Set("Deprecation", "true")
		if dep.SunsetDate != "" {
			w.Header().Set("Sunset", dep.SunsetDate)
		}
		if dep.ReplacementModel != "" {
			w.Header().Set("Link", fmt.Sprintf("</v1/models/%s>; rel=\"successor-version\"", dep.ReplacementModel))
		}
	}
}

// requestType 根据 isStream 返回请求类型
func requestType(isStream bool) int {
	if isStream {
		return 2
	}
	return 1
}

// relayModeToInboundFormat 根据 relay mode 推断入站请求格式
func relayModeToInboundFormat(mode constant.RelayMode) constant.RelayFormat {
	switch mode {
	case constant.RelayModeClaudeMessages:
		return constant.RelayFormatClaude
	case constant.RelayModeGeminiChat:
		return constant.RelayFormatGemini
	case constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		return constant.RelayFormatResponses
	default:
		return constant.RelayFormatOpenAI
	}
}

// relayModeString 转换 relay mode 为字符串
func relayModeString(mode constant.RelayMode) string {
	switch mode {
	case constant.RelayModeChatCompletions:
		return "chat_completions"
	case constant.RelayModeCompletions:
		return "completions"
	case constant.RelayModeEmbeddings:
		return "embeddings"
	case constant.RelayModeImagesGenerations:
		return "images_generations"
	case constant.RelayModeAudioSpeech:
		return "audio_speech"
	case constant.RelayModeAudioTranscription:
		return "audio_transcriptions"
	case constant.RelayModeAudioTranslation:
		return "audio_translations"
	case constant.RelayModeRerank:
		return "rerank"
	case constant.RelayModeResponses, constant.RelayModeResponsesCompact:
		return "responses"
	case constant.RelayModeRealtime:
		return "realtime"
	case constant.RelayModeClaudeMessages:
		return "claude_messages"
	case constant.RelayModeGeminiChat:
		return "gemini_generate_content"
	case constant.RelayModeModerations:
		return "moderations"
	case constant.RelayModeImagesEdits:
		return "images_edits"
	case constant.RelayModeMjSubmit:
		return "mj_submit"
	case constant.RelayModeMjFetch:
		return "mj_fetch"
	case constant.RelayModeMjImage:
		return "mj_image"
	case constant.RelayModeVideoGenerations:
		return "video_generations"
	default:
		return ""
	}
}
