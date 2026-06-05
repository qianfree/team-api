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

	commonlogic "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/monitor"
	"github.com/qianfree/team-api/relay/channel"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/helper"
	"github.com/qianfree/team-api/relay/override"
	"github.com/qianfree/team-api/relay/scheduler"
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
}

// validateRelayRequest 校验请求合法性：relay mode、模型存在性、弃用状态、成员/API Key 模型范围、QPS 限流。
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

	relayModeStr := relayModeString(relayMode)

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

	// 5. QPS 限流检查
	billingResult := &BillingResult{Deprecation: depInfo}
	if billing != nil {
		allowed, limitLevel, remaining, resetAt := billing.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID)
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
			Limit:     0, // 由 middleware 设置具体值
			Remaining: remaining,
			ResetAt:   resetAt,
		}
	}

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
	}, nil
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

	// 16.5 累加成员已用额度
	if billing != nil && settleResult != nil && settleResult.ActualCost > 0 {
		billing.IncrMemberQuotaUsed(postCtx, rc.TenantID, rc.UserID, settleResult.ActualCost)
	}

	// 17. 更新健康度 + 记录用量 + 更新亲和性
	provider.UpdateChannelHealth(postCtx, selection.ChannelID, true, info.LatencyMs())
	provider.ResetConsecutiveFailure(postCtx, selection.ChannelID)
	scheduler.GetGlobalAffinity().Set(rc.TenantID, rc.UserID, v.modelName, selection.ChannelID)

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
	// Phase 1: 校验请求（relay mode、模型、权限、限流）
	v, err := validateRelayRequest(ctx, body, path, rc, provider, billing)
	if err != nil {
		return nil, nil, err
	}

	// 在写入任何响应体之前设置限流/弃用 header（流式响应中 WriteHeader 后无法追加）
	setPreResponseHeaders(rc.Writer, v.billingResult)

	// Phase 2: 资源准备（并发控制 + 监控注册 + 预扣）
	if billing != nil {
		if !billing.AcquireConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, v.modelName) {
			return nil, nil, constant.NewRateLimitError("concurrent request limit exceeded")
		}
		defer billing.ReleaseConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, v.modelName)
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

	if billing != nil {
		if err := billing.CheckMemberQuota(ctx, rc.TenantID, rc.UserID, 0); err != nil {
			return nil, nil, constant.NewQuotaError("member quota exceeded", err)
		}
	}

	var preDeductAmount float64
	if billing != nil {
		amt, err := billing.PreDeduct(ctx, rc.TenantID, v.modelName, v.estimatedTokens, v.maxTokens, v.isStream, rc.RequestID)
		if err != nil {
			return nil, nil, constant.NewQuotaError("insufficient balance", err)
		}
		preDeductAmount = amt
		v.billingResult.PreDeductAmount = amt
	}

	// Phase 3: 带重试的渠道调度与请求执行
	excludeChannelIDs := make([]int64, 0)
	maxRetries := 3
	channelErrors := make([]string, 0)

	trace := &common.ForwardingTrace{
		EntryPath:      path,
		EntryFormat:    string(relayModeToInboundFormat(v.relayMode)),
		RequestedModel: v.modelName,
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		selection, err := provider.GetChannelForModel(ctx, rc.TenantID, rc.UserID, v.lookupModel, excludeChannelIDs)
		if err != nil {
			result := handleChannelUnavailable(ctx, billing, provider, rc, v, preDeductAmount, channelErrors, err)
			return result.usage, result.billingResult, result.err
		}

		info := buildRelayInfo(ctx, rc, v, selection, path, headers)

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

		upstreamCtx := context.WithoutCancel(ctx)
		settleCtx, settleCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer settleCancel()

		// 发送请求到上游
		resp, err := adaptor.DoRequest(upstreamCtx, info, convertedBody)
		if err != nil {
			failReason := fmt.Sprintf("attempt=%d channel=%d(%s) model=%s upstreamModel=%s error=[%v] latency=%.0fms retryable=%v",
				attempt, selection.ChannelID, selection.ChannelName, v.modelName, selection.UpstreamModelName, err, info.LatencyMs(), constant.IsRetryable(err))
			channelErrors = append(channelErrors, failReason)
			g.Log().Warningf(ctx, "[RelayHandler] Upstream request failed: %s", failReason)

			provider.UpdateChannelHealth(settleCtx, selection.ChannelID, false, info.LatencyMs())
			provider.IncrementConsecutiveFailure(settleCtx, selection.ChannelID)
			excludeChannelIDs = append(excludeChannelIDs, selection.ChannelID)
			scheduler.GetGlobalAffinity().Delete(rc.TenantID, rc.UserID, v.modelName)

			if constant.IsRetryable(err) && attempt < maxRetries {
				recordChannelError(rc, selection, v.modelName, attempt, false, err, info.LatencyMs())
				appendHop(trace, hop, false, err.Error(), info.LatencyMs())
				settleCancel()
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
		if err != nil {
			err = helper.RemapStatusCode(err, info.ChannelMeta.Settings.StatusCodeMapping)

			if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
				g.Log().Warningf(ctx, "[RelayHandler] Stream interrupted: adaptor=%s, model=%s, reason=%s",
					adaptor.GetChannelName(), v.modelName, info.StreamStatus.Summary())
				streamUsage := usage
				if streamUsage == nil {
					streamUsage = &common.Usage{}
				}
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleStreamInterrupted(settleCtx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
						v.modelName, rc.RequestID, v.relayModeStr, streamUsage, preDeductAmount, rc.ProjectID)
				}
				recordFailedUsage(provider, rc, selection.ChannelID, v.modelName, v.relayMode, v.isStream, err)
				finalizeTrace(trace, rc, hop, false, attempt, selection, err.Error(), info.LatencyMs())
				return usage, v.billingResult, err
			}

			g.Log().Errorf(ctx, "[RelayHandler] DoResponse failed: adaptor=%s, inboundFormat=%s, channel=%d(%s) model=%s attempt=%d error=%v latency=%.0fms",
				adaptor.GetChannelName(), info.InboundFormat, selection.ChannelID, selection.ChannelName, v.modelName, attempt, err, info.LatencyMs())
			provider.UpdateChannelHealth(settleCtx, selection.ChannelID, false, info.LatencyMs())
			provider.IncrementConsecutiveFailure(settleCtx, selection.ChannelID)
			excludeChannelIDs = append(excludeChannelIDs, selection.ChannelID)
			failReason := fmt.Sprintf("attempt=%d channel=%d(%s) model=%s doResponse_error=[%v] latency=%.0fms",
				attempt, selection.ChannelID, selection.ChannelName, v.modelName, err, info.LatencyMs())
			channelErrors = append(channelErrors, failReason)
			scheduler.GetGlobalAffinity().Delete(rc.TenantID, rc.UserID, v.modelName)

			if v.isStream || !constant.IsRetryable(err) || attempt >= maxRetries {
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(settleCtx, rc.TenantID, rc.RequestID, preDeductAmount)
				}
				recordFailedUsage(provider, rc, selection.ChannelID, v.modelName, v.relayMode, v.isStream, err)
				recordChannelError(rc, selection, v.modelName, attempt, true, err, info.LatencyMs())
				finalizeTrace(trace, rc, hop, false, attempt, selection, err.Error(), info.LatencyMs())
				return nil, v.billingResult, err
			}
			recordChannelError(rc, selection, v.modelName, attempt, false, err, info.LatencyMs())
			appendHop(trace, hop, false, err.Error(), info.LatencyMs())
			settleCancel()
			continue
		}

		// 成功路径
		appendHop(trace, hop, true, "", info.LatencyMs())
		trace.TotalAttempts = attempt + 1
		trace.UpstreamModel = selection.UpstreamModelName
		trace.ModelMapped = selection.IsModelMapped
		rc.ForwardingTrace = trace

		result := settleSuccessfulRequest(rc, v, usage, info, selection, preDeductAmount, provider, billing, headers, path)
		return usage, result, nil
	}

	// 不应到达此处
	if billing != nil && preDeductAmount > 0 {
		_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
	}
	return nil, v.billingResult, constant.ErrAllChannelsFailed
}

// buildRelayInfo 从渠道选择结果构建 RelayInfo
func buildRelayInfo(ctx context.Context, rc *RelayContext, v *relayValidation, selection *common.ChannelSelection, path string, headers http.Header) *common.RelayInfo {
	return &common.RelayInfo{
		Context:          ctx,
		TenantID:         rc.TenantID,
		UserID:           rc.UserID,
		ApiKeyID:         rc.ApiKeyID,
		ProjectID:        rc.ProjectID,
		RequestID:        rc.RequestID,
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
}

// convertRequestBody 根据是否直连模式转换请求体
func convertRequestBody(ctx context.Context, info *common.RelayInfo, body []byte, adaptor common.Adaptor) (io.Reader, error) {
	if canPassThrough(info) {
		if info.InboundFormat == constant.RelayFormatGemini {
			body = helper.StripStreamField(body)
		}
		return bytes.NewReader(body), nil
	}

	convertedBody, err := adaptor.ConvertRequest(ctx, info, body)
	if err != nil {
		g.Log().Errorf(ctx, "[RelayHandler] ConvertRequest failed: adaptor=%s, inboundFormat=%s, error=%v",
			adaptor.GetChannelName(), info.InboundFormat, err)
		return nil, err
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
) *channelUnavailableResult {
	if err != common.ErrChannelUnavailable {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		return &channelUnavailableResult{nil, v.billingResult, err}
	}

	if len(channelErrors) > 0 {
		if billing != nil && preDeductAmount > 0 {
			_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
		}
		g.Log().Errorf(ctx, "[RelayHandler] All %d channels failed for model=%s tenant=%d user=%d request=%s. Failure details: %s",
			len(channelErrors), v.modelName, rc.TenantID, rc.UserID, rc.RequestID, strings.Join(channelErrors, "\n"))
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
	g.Log().Errorf(ctx, "[RelayHandler] No available channel for model=%s tenant=%d user=%d", v.modelName, rc.TenantID, rc.UserID)
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
		Attempt:       attempt,
		ChannelID:     selection.ChannelID,
		ChannelName:   selection.ChannelName,
		ChannelType:   selection.ChannelType,
		Provider:      constant.ProviderType(selection.ChannelType).String(),
		BaseURL:       selection.BaseURL,
		UpstreamURL:   upstreamURL,
		UpstreamModel: selection.UpstreamModelName,
		ModelMapped:   selection.IsModelMapped,
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
	default:
		return ""
	}
}
