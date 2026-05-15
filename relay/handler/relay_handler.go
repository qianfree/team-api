package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gogf/gf/v2/frame/g"

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

// RelayHandler 共享的 relay 请求编排逻辑（带重试 + 计费）
func RelayHandler(ctx context.Context, body []byte, path string, headers http.Header, rc *RelayContext, provider common.DataProvider, billing common.BillingProvider) (*common.Usage, *BillingResult, error) {
	// 1. 确定 relay mode
	relayMode := constant.Path2RelayMode(path)
	if relayMode == constant.RelayModeUnknown {
		g.Log().Errorf(ctx, "[RelayHandler] Unknown relay mode for path: %s", path)
		return nil, nil, constant.NewRequestError("unsupported endpoint: "+path, nil)
	}

	relayModeStr := relayModeString(relayMode)

	// 2. 解析请求体获取模型名
	var rawRequest map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawRequest); err != nil {
		return nil, nil, constant.NewRequestError("invalid request body", err)
	}

	modelName := ""
	if v, ok := rawRequest["model"]; ok {
		var m string
		if err := json.Unmarshal(v, &m); err == nil {
			modelName = m
		}
	}
	if modelName == "" {
		return nil, nil, constant.NewRequestError("model is required", nil)
	}

	// 2.5 解析 thinking/effort 后缀
	thinkingInfo := helper.ParseThinkingSuffix(modelName)
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
			return nil, nil, constant.NewRequestError("model not found: "+modelName, modelErr)
		}
		return nil, nil, modelErr
	}

	// 3.5 检查模型弃用状态
	depInfo, _ := provider.GetModelDeprecationInfo(ctx, lookupModel)
	if depInfo != nil && depInfo.Deprecated && depInfo.SunsetDate != "" {
		sunsetTime, _ := time.Parse("2006-01-02", depInfo.SunsetDate)
		if !sunsetTime.IsZero() && time.Now().After(sunsetTime) {
			return nil, nil, constant.NewModelGoneError(lookupModel, depInfo.SunsetDate)
		}
	}

	// 3.7 检查成员模型范围
	if allowed, err := provider.CheckMemberModelAccess(ctx, rc.TenantID, rc.UserID, lookupModel); err != nil {
		return nil, nil, err
	} else if !allowed {
		return nil, nil, constant.NewAuthError("model not allowed for this member")
	}

	// 4. API Key 模型范围校验
	if allowed, err := provider.CheckApiKeyModelAccess(ctx, rc.ApiKeyID, lookupModel); err != nil {
		return nil, nil, err
	} else if !allowed {
		return nil, nil, constant.NewAuthError("model not allowed for this API key")
	}

	// 5. QPS 限流检查
	billingResult := &BillingResult{Deprecation: depInfo}
	if billing != nil {
		allowed, limitLevel, remaining, resetAt := billing.CheckRateLimit(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID)
		if !allowed {
			return nil, nil, &RelayErrorWithRateLimit{
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

	// 6. 并发限制（含模型级并发控制）
	if billing != nil {
		if !billing.AcquireConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, modelName) {
			return nil, nil, constant.NewRateLimitError("concurrent request limit exceeded")
		}
		defer billing.ReleaseConcurrent(ctx, rc.TenantID, rc.UserID, rc.ApiKeyID, modelName)
	}

	// 6.5 实时监控注册
	var isStream bool
	if streamVal, ok := rawRequest["stream"]; ok {
		_ = json.Unmarshal(streamVal, &isStream)
	}
	monitor.RegisterRequest(&monitor.TrackedRequest{
		RequestID: rc.RequestID,
		TenantID:  rc.TenantID,
		UserID:    rc.UserID,
		ProjectID: rc.ProjectID,
		ModelName: modelName,
		IsStream:  isStream,
		StartTime: time.Now(),
		Path:      path,
	})
	defer monitor.UnregisterRequest(rc.RequestID)

	// 7. 估算输入 token 数（粗略：按字符数 / 4）
	estimatedInputTokens := estimateInputTokens(body)

	// 获取 max_tokens
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

	// 7.5 成员额度预检查
	if billing != nil {
		if err := billing.CheckMemberQuota(ctx, rc.TenantID, rc.UserID, 0); err != nil {
			return nil, nil, constant.NewQuotaError("member quota exceeded", err)
		}
	}

	// 8. 预扣费用
	var preDeductAmount float64
	if billing != nil {
		amt, err := billing.PreDeduct(ctx, rc.TenantID, modelName, estimatedInputTokens, maxTokens, isStream, rc.RequestID)
		if err != nil {
			return nil, nil, constant.NewQuotaError("insufficient balance", err)
		}
		preDeductAmount = amt
		billingResult.PreDeductAmount = amt
	}

	// 10. 带重试的请求调度
	excludeChannelIDs := make([]int64, 0)
	maxRetries := 3
	originalBody := body // 保存原始请求体，避免重试时重复转换

	// 初始化转发路径追踪
	trace := &common.ForwardingTrace{
		EntryPath:      path,
		EntryFormat:    string(relayModeToInboundFormat(relayMode)),
		RequestedModel: modelName,
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// 选择渠道（排除已失败的）
		selection, err := provider.GetChannelForModel(ctx, rc.TenantID, rc.UserID, lookupModel, excludeChannelIDs)
		if err != nil {
			if err == common.ErrChannelUnavailable {
				if len(excludeChannelIDs) > 0 {
					// 所有渠道失败，退还预扣
					if billing != nil && preDeductAmount > 0 {
						_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
					}
					return nil, billingResult, constant.NewChannelError(
						fmt.Sprintf("all %d channels failed for model: %s", len(excludeChannelIDs), modelName),
						constant.ErrAllChannelsFailed,
					)
				}
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
				}
				return nil, billingResult, constant.NewChannelError("no available channel for model: "+modelName, err)
			}
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, billingResult, err
		}

		// 11. 构建 RelayInfo
		info := &common.RelayInfo{
			Context:          ctx,
			TenantID:         rc.TenantID,
			UserID:           rc.UserID,
			ApiKeyID:         rc.ApiKeyID,
			ProjectID:        rc.ProjectID,
			RequestID:        rc.RequestID,
			RelayMode:        int(relayMode),
			IsStream:         isStream,
			OriginModelName:  modelName,
			BaseModelName:    lookupModel,
			ThinkingEnabled:  thinkingInfo.IsThinking,
			ThinkingDisabled: thinkingInfo.IsNoThinking,
			ReasoningEffort:  thinkingInfo.EffortLevel,
			RequestURLPath:   path,
			RequestHeaders:   headers,
			StartTime:        time.Now(),
			StreamStatus:     common.NewStreamStatus(),
			InboundFormat:    relayModeToInboundFormat(relayMode),
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

		// 11.5 更新实时监控中的渠道信息
		if tr := monitor.GetTrackedRequest(rc.RequestID); tr != nil {
			tr.ChannelID = selection.ChannelID
			tr.ChannelName = selection.ChannelName
		}

		// 12. 获取适配器
		adaptor := channel.GetAdaptor(selection.ChannelType)
		if adaptor == nil {
			g.Log().Errorf(ctx, "[RelayHandler] No adaptor found for channelType: %d", selection.ChannelType)
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
			}
			return nil, billingResult, fmt.Errorf("unsupported channel type: %d", selection.ChannelType)
		}
		adaptor.Init(info)

		// 捕获上游 URL 用于转发追踪
		var upstreamURL string
		if u, err := adaptor.GetRequestURL(info); err == nil {
			upstreamURL = u
		}
		hop := common.ForwardingHop{
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

		// 12.5 保存客户端原始格式（用于响应方向转换）
		// 每次重试从原始请求体开始转换，避免重复转换
		workingBody := originalBody
		if info.ClientFormat == "" {
			info.ClientFormat = info.InboundFormat
		}

		// 13. 转换请求（直连模式跳过协议转换和参数改写）
		var convertedBody io.Reader
		if canPassThrough(info) {
			// Gemini 原生格式通过 URL 路径控制流式，body 中的 "stream" 字段会导致上游报错
			if info.InboundFormat == constant.RelayFormatGemini {
				workingBody = helper.StripStreamField(workingBody)
			}
			convertedBody = bytes.NewReader(workingBody)
		} else {
			var err error
			convertedBody, err = adaptor.ConvertRequest(ctx, info, workingBody)
			if err != nil {
				g.Log().Errorf(ctx, "[RelayHandler] ConvertRequest failed: adaptor=%s, inboundFormat=%s, error=%v",
					adaptor.GetChannelName(), info.InboundFormat, err)
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
				}
				return nil, billingResult, err
			}

			// 13.3 注入渠道系统提示词
			if info.ChannelMeta.Settings.SystemPrompt != "" {
				bodyBytes, _ := io.ReadAll(convertedBody)
				bodyBytes = helper.InjectSystemPrompt(bodyBytes, info)
				convertedBody = bytes.NewReader(bodyBytes)
			}

			// 13.5 应用请求体改写（ParamOverride）
			if info.ChannelMeta.Settings.ParamOverride != nil {
				bodyBytes, err := io.ReadAll(convertedBody)
				if err != nil {
					g.Log().Errorf(ctx, "[RelayHandler] Read converted body for param override failed: %v", err)
					if billing != nil && preDeductAmount > 0 {
						_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
					}
					return nil, billingResult, err
				}
				bodyBytes, err = override.ApplyParamOverride(bodyBytes, info)
				if err != nil {
					if retErr, ok := override.AsReturnError(err); ok {
						return nil, billingResult, constant.NewUpstreamError(retErr.StatusCode, retErr.Message, retErr)
					}
					return nil, billingResult, err
				}
				convertedBody = bytes.NewReader(bodyBytes)
			}

			// 13.7 字段清理
			{
				sanitized, _ := io.ReadAll(convertedBody)
				sanitized = helper.SanitizeFields(sanitized, info.ChannelMeta.Settings)
				convertedBody = bytes.NewReader(sanitized)
			}
		}

		// 13.9 上游请求 context 解耦：客户端断开不应中断上游请求，
		// 参考 new-api/sub2api：上游请求用 WithoutCancel 解耦，超时由 HTTP Client 和 StreamScanner 控制。
		upstreamCtx := context.WithoutCancel(ctx)

		// 独立结算 context（加超时保护，防止结算操作无限阻塞）
		settleCtx, settleCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer settleCancel()

		// 14. 发送请求到上游
		resp, err := adaptor.DoRequest(upstreamCtx, info, convertedBody)
		if err != nil {
			provider.UpdateChannelHealth(settleCtx, selection.ChannelID, false, info.LatencyMs())
			provider.IncrementConsecutiveFailure(settleCtx, selection.ChannelID)
			excludeChannelIDs = append(excludeChannelIDs, selection.ChannelID)

			// 亲和性渠道失败，清除亲和性
			scheduler.GetGlobalAffinity().Delete(rc.TenantID, rc.UserID, modelName)

			if constant.IsRetryable(err) && attempt < maxRetries {
				hop.Success = false
				hop.Error = err.Error()
				hop.LatencyMs = info.LatencyMs()
				trace.Hops = append(trace.Hops, hop)
				continue
			}

			// 不可重试或达到上限：退还预扣
			if billing != nil && preDeductAmount > 0 {
				_ = billing.SettleFailed(settleCtx, rc.TenantID, rc.RequestID, preDeductAmount)
			}

			recordFailedUsage(provider, rc, selection.ChannelID, modelName, relayMode, isStream, err)

			hop.Success = false
			hop.Error = err.Error()
			hop.LatencyMs = info.LatencyMs()
			trace.Hops = append(trace.Hops, hop)
			trace.TotalAttempts = attempt + 1
			trace.UpstreamModel = selection.UpstreamModelName
			trace.ModelMapped = selection.IsModelMapped
			rc.ForwardingTrace = trace

			return nil, billingResult, helper.RemapStatusCode(constant.NewUpstreamError(502, "upstream request failed", err), info.ChannelMeta.Settings.StatusCodeMapping)
		}

		// 非流式：DoRequest 返回即表示收到上游首响应，设置首Token时间
		if !isStream {
			info.SetFirstResponseTime()
		}

		// 15. 处理响应
		usage, err := adaptor.DoResponse(ctx, resp, info, rc.Writer)
		if err != nil {
			err = helper.RemapStatusCode(err, info.ChannelMeta.Settings.StatusCodeMapping)

			// 流式中断（客户端断开、上游超时等）：降级为 WARN，不影响渠道健康度
			if info.StreamStatus != nil && info.StreamStatus.IsPartialStreamEnd() {
				g.Log().Warningf(ctx, "[RelayHandler] Stream interrupted: adaptor=%s, model=%s, reason=%s",
					adaptor.GetChannelName(), modelName, info.StreamStatus.Summary())

				streamUsage := usage
				if streamUsage == nil {
					streamUsage = &common.Usage{}
				}
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleStreamInterrupted(settleCtx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
						modelName, rc.RequestID, relayModeStr, streamUsage, preDeductAmount)
				}
				recordFailedUsage(provider, rc, selection.ChannelID, modelName, relayMode, isStream, err)
				hop.Success = false
				hop.Error = err.Error()
				hop.LatencyMs = info.LatencyMs()
				trace.Hops = append(trace.Hops, hop)
				trace.TotalAttempts = attempt + 1
				trace.UpstreamModel = selection.UpstreamModelName
				trace.ModelMapped = selection.IsModelMapped
				rc.ForwardingTrace = trace
				return usage, billingResult, err
			}

			// 非中断类错误：记录 ERROR 并更新渠道健康度
			g.Log().Errorf(ctx, "[RelayHandler] DoResponse failed: adaptor=%s, inboundFormat=%s, error=%v",
				adaptor.GetChannelName(), info.InboundFormat, err)
			provider.UpdateChannelHealth(settleCtx, selection.ChannelID, false, info.LatencyMs())
			provider.IncrementConsecutiveFailure(settleCtx, selection.ChannelID)
			excludeChannelIDs = append(excludeChannelIDs, selection.ChannelID)

			// 亲和性渠道失败，清除亲和性
			scheduler.GetGlobalAffinity().Delete(rc.TenantID, rc.UserID, modelName)

			// 其他失败
			if isStream || !constant.IsRetryable(err) || attempt >= maxRetries {
				// 退还预扣
				if billing != nil && preDeductAmount > 0 {
					_ = billing.SettleFailed(settleCtx, rc.TenantID, rc.RequestID, preDeductAmount)
				}
				recordFailedUsage(provider, rc, selection.ChannelID, modelName, relayMode, isStream, err)
				hop.Success = false
				hop.Error = err.Error()
				hop.LatencyMs = info.LatencyMs()
				trace.Hops = append(trace.Hops, hop)
				trace.TotalAttempts = attempt + 1
				trace.UpstreamModel = selection.UpstreamModelName
				trace.ModelMapped = selection.IsModelMapped
				rc.ForwardingTrace = trace

				return nil, billingResult, err
			}
			hop.Success = false
			hop.Error = err.Error()
			hop.LatencyMs = info.LatencyMs()
			trace.Hops = append(trace.Hops, hop)
			continue
		}

		hop.Success = true
		hop.LatencyMs = info.LatencyMs()
		trace.Hops = append(trace.Hops, hop)
		trace.TotalAttempts = attempt + 1
		trace.UpstreamModel = selection.UpstreamModelName
		trace.ModelMapped = selection.IsModelMapped
		rc.ForwardingTrace = trace

		// 16. 成功：结算费用（使用完整 Usage 含 cache token）
		// 流式响应完成后客户端可能已断开，ctx 已取消，
		// 结算/健康度/用量记录等后置操作必须使用独立 context，不受请求生命周期影响。
		// 重要：此处重新创建 context，因为上游 DoResponse 可能耗时很长（长文本流式输出），
		// 循环开头创建的 settleCtx 此时可能已经过期。
		postCtx, postCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer postCancel()

		var settleResult *common.SettlementResult
		if billing != nil && preDeductAmount > 0 {
			settleResult = billing.SettleWithUsage(postCtx, rc.TenantID, rc.UserID, rc.ApiKeyID, selection.ChannelID,
				modelName, rc.RequestID, relayModeStr,
				usage, preDeductAmount, info)
			if settleResult != nil {
				billingResult.ActualCost = settleResult.ActualCost
			} else {
				// 结算失败，退还预扣冻结金额
				g.Log().Warningf(postCtx, "[RelayHandler] Settlement failed for request=%s model=%s, refunding pre-deduct amount=%.6f",
					rc.RequestID, modelName, preDeductAmount)
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

		scheduler.GetGlobalAffinity().Set(rc.TenantID, rc.UserID, modelName, selection.ChannelID)

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
			ModelName:        modelName,
			RelayMode:        int(relayMode),
			PromptTokens:     usage.PromptTokens,
			CompletionTokens: usage.CompletionTokens,
			TotalTokens:      usage.TotalTokens,
			CachedTokens:     tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.CachedTokens }),
			AudioTokens: tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.AudioTokens }) +
				tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.AudioTokens }),
			ImageTokens:     tokenDetailField(usage.PromptTokensDetails, func(d *common.TokenDetails) int { return d.ImageTokens }),
			ReasoningTokens: tokenDetailField(usage.CompletionTokenDetails, func(d *common.TokenDetails) int { return d.ReasoningTokens }),
			LatencyMs:       info.TotalLatencyMs(),
			IsStream:        isStream,
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
			RequestedModel:  modelName,
			UpstreamModel:   selection.UpstreamModelName,
			RequestType:     requestType(isStream),
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

		billingResult.FirstTokenMs = firstTokenMs

		provider.RecordUsage(postCtx, usageRecord)

		return usage, billingResult, nil
	}

	// 不应到达此处
	if billing != nil && preDeductAmount > 0 {
		_ = billing.SettleFailed(ctx, rc.TenantID, rc.RequestID, preDeductAmount)
	}
	return nil, billingResult, constant.ErrAllChannelsFailed
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
