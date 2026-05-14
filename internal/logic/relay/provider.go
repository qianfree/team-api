package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
	loauth "github.com/qianfree/team-api/internal/logic/common/oauth"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/scheduler"
)

// modelCache 模型信息缓存（TTL 600s）
var modelCache = lcommon.NewCache("model", 600*time.Second)

// DataProviderImpl DataProvider 接口的 GoFrame ORM 实现
type DataProviderImpl struct{}

// NewDataProvider 创建 DataProvider 实例
func NewDataProvider() common.DataProvider {
	return &DataProviderImpl{}
}

// ValidateApiKey 实现 DataProvider.ValidateApiKey
func (p *DataProviderImpl) ValidateApiKey(ctx context.Context, rawKey string) (*common.ApiKeyInfo, error) {
	info, err := ValidateApiKey(ctx, rawKey)
	if err != nil {
		return nil, err
	}
	return &common.ApiKeyInfo{
		ID:        info.ID,
		TenantID:  info.TenantID,
		UserID:    info.UserID,
		ProjectID: info.ProjectID,
		Scope:     info.Scope,
		Status:    info.Status,
	}, nil
}

// GetChannelForModel 实现 DataProvider.GetChannelForModel
// 使用调度引擎：亲和性优先 → 租户模型权限 → 渠道范围过滤 → 优先级+权重选择 → 健康度过滤
func (p *DataProviderImpl) GetChannelForModel(ctx context.Context, tenantID, userID int64, modelName string, excludeChannelIDs []int64) (*common.ChannelSelection, error) {
	// 检查租户模型权限和渠道范围
	enabled, channelScope, err := p.CheckTenantModelAccess(ctx, tenantID, modelName)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, common.ErrTenantModelNotEnabled
	}

	// 亲和性查找：优先使用上次成功的渠道，保持会话连续性
	if preferredChannelID, ok := scheduler.GetGlobalAffinity().Get(tenantID, userID, modelName); ok {
		if selection, err := p.tryAffinityChannel(ctx, tenantID, modelName, preferredChannelID, channelScope, excludeChannelIDs); err == nil && selection != nil {
			return selection, nil
		}
		// 亲和性渠道不可用，清除记录，继续正常调度
		scheduler.GetGlobalAffinity().Delete(tenantID, userID, modelName)
	}

	return p.selectChannelFromDB(ctx, tenantID, modelName, channelScope, excludeChannelIDs)
}

// tryAffinityChannel 尝试使用亲和性渠道
func (p *DataProviderImpl) tryAffinityChannel(ctx context.Context, tenantID int64, modelName string, preferredChannelID int64, channelScope []int64, excludeChannelIDs []int64) (*common.ChannelSelection, error) {
	// 检查是否在排除列表中
	for _, id := range excludeChannelIDs {
		if id == preferredChannelID {
			return nil, common.ErrChannelUnavailable
		}
	}

	// 检查渠道范围
	if len(channelScope) > 0 {
		found := false
		for _, id := range channelScope {
			if id == preferredChannelID {
				found = true
				break
			}
		}
		if !found {
			return nil, common.ErrChannelUnavailable
		}
	}

	type channelRow struct {
		ChannelID     int64  `json:"channel_id"`
		ChannelName   string `json:"channel_name"`
		ChannelType   int    `json:"channel_type"`
		BaseURL       string `json:"base_url"`
		UpstreamModel string `json:"upstream_model"`
		Settings      string `json:"settings"`
	}

	var ch *channelRow
	err := dao.ChnChannels.Ctx(ctx).As("c").
		LeftJoin("chn_abilities a ON a.channel_id = c.id").
		Where("c.id", preferredChannelID).
		Where("c.status", "active").
		Where("a.model_name", modelName).
		Where("a.enabled", true).
		Fields("c.id as channel_id, c.name as channel_name, c.type as channel_type, c.base_url, a.upstream_model, c.settings").
		Scan(&ch)
	if err != nil || ch == nil {
		return nil, common.ErrChannelUnavailable
	}

	key, err := getChannelKey(ctx, ch.ChannelID)
	if err != nil || key == "" {
		return nil, common.ErrChannelUnavailable
	}

	settings := ParseChannelSettings(ch.Settings)
	upstreamModel := ch.UpstreamModel
	if upstreamModel == "" {
		upstreamModel = modelName
	}

	return &common.ChannelSelection{
		ChannelID:         ch.ChannelID,
		ChannelType:       ch.ChannelType,
		ChannelName:       ch.ChannelName,
		BaseURL:           ch.BaseURL,
		ApiKey:            key,
		UpstreamModelName: upstreamModel,
		IsModelMapped:     ch.UpstreamModel != "" && ch.UpstreamModel != modelName,
		Settings:          settings,
	}, nil
}

// selectChannelFromDB 正常渠道调度（优先级+权重+健康度）
func (p *DataProviderImpl) selectChannelFromDB(ctx context.Context, tenantID int64, modelName string, channelScope []int64, excludeChannelIDs []int64) (*common.ChannelSelection, error) {
	type channelRow struct {
		ChannelID           int64    `json:"channel_id"`
		ChannelName         string   `json:"channel_name"`
		ChannelType         int      `json:"channel_type"`
		BaseURL             string   `json:"base_url"`
		UpstreamModel       string   `json:"upstream_model"`
		Priority            int      `json:"priority"`
		Weight              int      `json:"weight"`
		Settings            string   `json:"settings"`
		HealthScore         *float64 `json:"health_score"`
		ConsecutiveFailures int      `json:"consecutive_failures"`
	}

	query := dao.ChnAbilities.Ctx(ctx).As("a").
		LeftJoin("chn_channels c ON a.channel_id = c.id").
		LeftJoin("chn_health_scores h ON c.id = h.channel_id").
		Where("a.model_name", modelName).
		Where("a.enabled", true).
		Where("c.status", "active").
		Fields("c.id as channel_id, c.name as channel_name, c.type as channel_type, c.base_url, a.upstream_model, c.priority, c.weight, c.settings, h.health_score, h.consecutive_failures").
		OrderDesc("c.priority").
		OrderDesc("c.weight")

	// 渠道范围过滤
	if len(channelScope) > 0 {
		query = query.WhereIn("c.id", channelScope)
	}

	for _, id := range excludeChannelIDs {
		query = query.WhereNot("c.id", id)
	}

	var channels []channelRow
	err := query.Scan(&channels)
	if err != nil {
		return nil, err
	}

	if len(channels) == 0 {
		return nil, common.ErrChannelUnavailable
	}

	// 尝试按 last_used_at 选择最早使用的 Key
	for _, ch := range channels {
		key, err := getChannelKey(ctx, ch.ChannelID)
		if err == nil && key != "" {
			settings := ParseChannelSettings(ch.Settings)

			upstreamModel := ch.UpstreamModel
			if upstreamModel == "" {
				upstreamModel = modelName
			}

			return &common.ChannelSelection{
				ChannelID:         ch.ChannelID,
				ChannelType:       ch.ChannelType,
				ChannelName:       ch.ChannelName,
				BaseURL:           ch.BaseURL,
				ApiKey:            key,
				UpstreamModelName: upstreamModel,
				IsModelMapped:     ch.UpstreamModel != "" && ch.UpstreamModel != modelName,
				Settings:          settings,
			}, nil
		}
	}

	return nil, common.ErrChannelUnavailable
}

// CheckTenantModelAccess 实现 DataProvider.CheckTenantModelAccess
// 检查租户是否有权使用指定模型，返回是否启用和渠道范围
func (p *DataProviderImpl) CheckTenantModelAccess(ctx context.Context, tenantID int64, modelName string) (bool, []int64, error) {
	type accessRow struct {
		Enabled      bool   `json:"enabled"`
		ChannelScope string `json:"channel_scope"`
	}

	var row *accessRow
	err := dao.MdlTenantModels.Ctx(ctx).As("tm").
		LeftJoin("mdl_models m ON tm.model_id = m.id").
		Where("tm.tenant_id", tenantID).
		Where("m.model_id", modelName).
		Fields("tm.enabled, tm.channel_scope").
		Scan(&row)
	if err != nil {
		return false, nil, err
	}

	// 如果没有分配记录，默认允许（向后兼容：未分配模型时所有活跃模型可用）
	if row == nil {
		return true, nil, nil
	}
	if !row.Enabled && row.ChannelScope == "" {
		return false, nil, nil
	}

	// 解析渠道范围 JSONB
	var scope []int64
	if row.ChannelScope != "" && row.ChannelScope != "[]" && row.ChannelScope != "null" {
		_ = json.Unmarshal([]byte(row.ChannelScope), &scope)
	}

	return row.Enabled, scope, nil
}

// GetModelMapping 实现 DataProvider.GetModelMapping
func (p *DataProviderImpl) GetModelMapping(ctx context.Context, modelName string) (string, string, error) {
	cacheKey := modelName
	var cached modelInfoCached
	if modelCache.GetJSON(ctx, cacheKey, &cached) {
		return cached.StandardName, cached.Category, nil
	}

	type modelRow struct {
		ModelId  string `json:"model_id"`
		Category string `json:"category"`
		Status   string `json:"status"`
	}

	var model modelRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("model_id, category, status").
		Scan(&model)
	if err != nil {
		// sql.ErrNoRows 表示模型不存在，转换为业务错误
		if strings.Contains(err.Error(), "no rows in result set") {
			return "", "", common.ErrModelNotFound
		}
		return "", "", err
	}
	if model.ModelId == "" {
		return "", "", common.ErrModelNotFound
	}
	if model.Status == "offline" {
		return "", "", common.ErrModelNotFound
	}
	// deprecated 模型放行，弃用信息由 GetModelDeprecationInfo 获取

	info := &modelInfoCached{
		StandardName: model.ModelId,
		Category:     model.Category,
	}
	modelCache.Set(ctx, cacheKey, info)

	return model.ModelId, model.Category, nil
}

// RecordUsage 实现 DataProvider.RecordUsage
func (p *DataProviderImpl) RecordUsage(ctx context.Context, record *common.UsageRecord) {
	lcommon.DefaultUsageLogWriter.Submit(buildUsageLogDO(record))
}

// buildUsageLogDO 将 UsageRecord 转换为 DO 对象
func buildUsageLogDO(record *common.UsageRecord) do.BilUsageLogs {
	requestType := record.RequestType
	if requestType == 0 {
		if record.IsStream {
			requestType = 2
		} else {
			requestType = 1
		}
	}

	return do.BilUsageLogs{
		TenantId:     record.TenantID,
		UserId:       record.UserID,
		ApiKeyId:     record.ApiKeyID,
		ProjectId:    record.ProjectID,
		ChannelId:    record.ChannelID,
		ModelName:    record.ModelName,
		RequestId:    record.RequestID,
		RelayMode:    record.RelayMode,
		InputTokens:  record.PromptTokens,
		OutputTokens: record.CompletionTokens,
		TotalCost:    record.TotalCost,
		LatencyMs:    int(record.LatencyMs),
		Status:       record.Status,
		ErrorMessage: record.ErrorMessage,
		ClientIp:     record.ClientIP,

		InputCost:         record.InputCost,
		OutputCost:        record.OutputCost,
		CacheCreationCost: record.CacheCreationCost,
		CacheReadCost:     record.CacheReadCost,
		ActualCost:        record.ActualCost,
		Currency:          record.Currency,
		PreDeductAmount:   record.PreDeductAmount,
		RefundAmount:      record.RefundAmount,
		SupplementAmount:  record.SupplementAmount,

		CacheCreationTokens: record.CacheCreationTokens,
		CacheReadTokens:     record.CacheReadTokens,

		AudioInputTokens:  record.AudioInputTokens,
		AudioOutputTokens: record.AudioOutputTokens,
		ImageOutputTokens: record.ImageOutputTokens,
		ReasoningTokens:   record.ReasoningTokens,

		RequestedModel: record.RequestedModel,
		UpstreamModel:  record.UpstreamModel,

		RequestType:     requestType,
		UserAgent:       record.UserAgent,
		FirstTokenMs:    record.FirstTokenMs,
		ServiceTier:     record.ServiceTier,
		ReasoningEffort: record.ReasoningEffort,
		InboundEndpoint: record.InboundEndpoint,

		ChannelName: record.ChannelName,
		ChannelType: record.ChannelType,

		BillingMode:    record.BillingMode,
		BillingSource:  record.BillingSource,
		RateMultiplier: record.RateMultiplier,
		RetryIndex:     record.RetryIndex,

		StreamEndReason: record.StreamEndReason,

		ImageCount: record.ImageCount,
		ImageSize:  record.ImageSize,

		BillingSnapshot: jsonNullIfEmpty(record.BillingSnapshot),
		BillingSummary:  record.BillingSummary,
	}
}

// RecordAudit 实现 DataProvider.RecordAudit
// 异步写入请求审计日志，同时按系统级别和租户级别分别处理请求/响应体
func (p *DataProviderImpl) RecordAudit(ctx context.Context, record *common.AuditRecord) {
	go func() {
		// 防止 panic 导致 goroutine 静默退出
		defer func() {
			if r := recover(); r != nil {
				g.Log().Errorf(context.Background(),
					"record audit log panic: request_id=%s tenant_id=%d path=%s panic=%v",
					record.RequestID, record.TenantID, record.Path, r)
			}
		}()

		bgCtx := context.Background()

		// 分别获取全局级别和租户级别
		globalLevel, tenantLevel := lcommon.GetAuditLevels(bgCtx, record.TenantID)

		// 两级都是 none 时不记录
		if globalLevel == lcommon.AuditLevelNone && tenantLevel == lcommon.AuditLevelNone {
			return
		}

		// 按系统级别处理
		sysReq, sysResp := lcommon.ApplyAuditLevel(globalLevel, record.RequestBody, record.ResponseBody, record.IsStream, record.Path)
		// 按租户级别处理
		tntReq, tntResp := lcommon.ApplyAuditLevel(tenantLevel, record.RequestBody, record.ResponseBody, record.IsStream, record.Path)

		// 截断过长的内容
		maxBodyLen := 65536
		sysReq, sysResp = truncateBody(sysReq, maxBodyLen), truncateBody(sysResp, maxBodyLen)
		tntReq, tntResp = truncateBody(tntReq, maxBodyLen), truncateBody(tntResp, maxBodyLen)

		// 仅审计级别为 all 时记录请求头和响应头
		var requestHeadersJSON, responseHeadersJSON string
		if globalLevel == lcommon.AuditLevelFull {
			if record.RequestHeaders != nil {
				b, _ := json.Marshal(record.RequestHeaders)
				requestHeadersJSON = string(b)
			}
			if record.ResponseHeaders != nil {
				b, _ := json.Marshal(record.ResponseHeaders)
				responseHeadersJSON = string(b)
			}
		}

		// 序列化转发路径追踪
		var forwardingTraceJSON string
		if record.ForwardingTrace != nil {
			if b, err := json.Marshal(record.ForwardingTrace); err == nil {
				forwardingTraceJSON = string(b)
			}
		}

		insertData := g.Map{
			"tenant_id":            record.TenantID,
			"user_id":              record.UserID,
			"api_key_id":           record.ApiKeyID,
			"project_id":           record.ProjectID,
			"request_id":           record.RequestID,
			"method":               record.Method,
			"path":                 record.Path,
			"query_params":         record.QueryParams,
			"status_code":          record.StatusCode,
			"client_ip":            record.ClientIP,
			"user_agent":           record.UserAgent,
			"request_body":         sysReq,
			"response_body":        sysResp,
			"tenant_request_body":  tntReq,
			"tenant_response_body": tntResp,
			"latency_ms":           record.LatencyMs,
			"first_token_ms":       record.FirstTokenMs,
			"audit_level":          globalLevel,
			"tenant_audit_level":   tenantLevel,
			"request_headers":      requestHeadersJSON,
			"response_headers":     responseHeadersJSON,
			"forwarding_trace":     forwardingTraceJSON,
		}
		_, insertErr := dao.AudRequestLogs.Ctx(bgCtx).Data(insertData).Insert()
		if insertErr != nil {
			g.Log().Errorf(bgCtx,
				"record audit log failed: request_id=%s tenant_id=%d api_key_id=%d path=%s status=%d err=%v",
				record.RequestID, record.TenantID, record.ApiKeyID, record.Path, record.StatusCode, insertErr)
		}
	}()
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// 流式响应：保留首部 + 最后 20 条消息，截断中间
	if strings.Contains(s, "\ndata:") || strings.HasPrefix(s, "data:") {
		return truncateStreamBody(s, maxLen)
	}
	return safeUTF8Truncate(s, maxLen) + "\n...[truncated]"
}

// truncateStreamBody 截断流式响应内容，保留首部和尾部消息（包含 usage/finish 等关键信息）
func truncateStreamBody(s string, maxLen int) string {
	const keepTailLines = 20
	const marker = "\n...[truncated]...\n"

	lines := strings.Split(s, "\n")
	if len(lines) <= keepTailLines {
		return safeUTF8Truncate(s, maxLen) + "\n...[truncated]"
	}

	tail := strings.Join(lines[len(lines)-keepTailLines:], "\n")
	headBudget := maxLen - len(marker) - len(tail)
	if headBudget <= 0 {
		if len(tail) > maxLen {
			return safeUTF8Truncate(tail, maxLen) + "\n...[truncated]"
		}
		return tail
	}

	head := s
	if len(head) > headBudget {
		head = head[:headBudget]
		if idx := strings.LastIndexByte(head, '\n'); idx > 0 {
			head = head[:idx]
		}
	}

	return head + marker + tail
}

// safeUTF8Truncate 按字节截断字符串，确保不在多字节 UTF-8 字符中间截断
func safeUTF8Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	for maxLen > 0 && !utf8.RuneStart(s[maxLen]) {
		maxLen--
	}
	return s[:maxLen]
}

// jsonNullIfEmpty 将空字符串转为 nil，避免空字符串写入 JSONB 列报错。
func jsonNullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// UpdateChannelHealth 实现 DataProvider.UpdateChannelHealth
func (p *DataProviderImpl) UpdateChannelHealth(ctx context.Context, channelID int64, success bool, latencyMs float64) {
	go func() {
		bgCtx := context.Background()
		UpdateHealthScore(bgCtx, channelID, success, latencyMs)
	}()
}

// IncrementConsecutiveFailure 实现 DataProvider.IncrementConsecutiveFailure
func (p *DataProviderImpl) IncrementConsecutiveFailure(ctx context.Context, channelID int64) {
	incrementConsecutiveFailure(ctx, channelID)
}

// ResetConsecutiveFailure 实现 DataProvider.ResetConsecutiveFailure
func (p *DataProviderImpl) ResetConsecutiveFailure(ctx context.Context, channelID int64) {
	dao.ChnHealthScores.Ctx(ctx).
		Where("channel_id", channelID).
		Data(do.ChnHealthScores{
			ConsecutiveFailures: 0,
		}).Update()
}

// GetAvailableModels 实现 DataProvider.GetAvailableModels
// 如果 tenantID > 0，返回该租户有权使用的模型列表
// 如果 apiKeyID > 0，进一步按 API Key 的模型范围过滤
func (p *DataProviderImpl) GetAvailableModels(ctx context.Context, tenantID int64, apiKeyID int64) ([]common.ModelInfo, error) {
	type modelRow struct {
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		Status           string `json:"status"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Capabilities     string `json:"capabilities"`
	}

	var models []modelRow
	var err error

	fields := "m.model_id, m.model_name, m.category, m.status, m.max_context_tokens, m.max_output_tokens, m.capabilities"
	fieldsNoAlias := "model_id, model_name, category, status, max_context_tokens, max_output_tokens, capabilities"

	if tenantID > 0 {
		// 检查租户是否有模型分配记录
		count, _ := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", tenantID).
			Count()

		if count > 0 {
			// 有分配记录：只返回租户启用的模型
			err = g.DB().Model("mdl_models m").Ctx(ctx).
				InnerJoin("mdl_tenant_models tm ON tm.model_id = m.id").
				Where("tm.tenant_id", tenantID).
				Where("tm.enabled", true).
				Where("m.status", "active").
				Fields(fields).
				OrderAsc("m.category").
				OrderAsc("m.model_id").
				Scan(&models)
		} else {
			// 无分配记录：返回所有活跃模型（向后兼容）
			err = dao.MdlModels.Ctx(ctx).
				Where("status", "active").
				Fields(fieldsNoAlias).
				OrderAsc("category").
				OrderAsc("model_id").
				Scan(&models)
		}
	} else {
		// tenantID == 0：返回所有活跃模型（公开端点场景）
		err = dao.MdlModels.Ctx(ctx).
			Where("status", "active").
			Fields(fieldsNoAlias).
			OrderAsc("category").
			OrderAsc("model_id").
			Scan(&models)
	}

	if err != nil {
		return nil, err
	}

	// 按 API Key 的模型范围过滤
	if apiKeyID > 0 {
		var keyScopes []struct {
			ModelName string `json:"model_name"`
		}
		_ = dao.ApiKeyModelScopes.Ctx(ctx).
			Where("api_key_id", apiKeyID).
			Fields("model_name").
			Scan(&keyScopes)

		if len(keyScopes) > 0 {
			allowed := make(map[string]bool, len(keyScopes))
			for _, s := range keyScopes {
				allowed[s.ModelName] = true
			}
			filtered := make([]modelRow, 0, len(models))
			for _, m := range models {
				if allowed[m.ModelId] {
					filtered = append(filtered, m)
				}
			}
			models = filtered
		}
	}

	result := make([]common.ModelInfo, 0, len(models))
	for _, m := range models {
		result = append(result, common.ModelInfo{
			ModelId:          m.ModelId,
			ModelName:        m.ModelName,
			Category:         m.Category,
			Status:           m.Status,
			MaxContextTokens: m.MaxContextTokens,
			MaxOutputTokens:  m.MaxOutputTokens,
			Capabilities:     parseCapabilitiesJSON(m.Capabilities),
		})
	}

	return result, nil
}

// GetModelDetail 实现 DataProvider.GetModelDetail
// 获取单个模型的详细信息，同时校验租户权限
func (p *DataProviderImpl) GetModelDetail(ctx context.Context, tenantID int64, modelName string) (*common.ModelDetail, error) {
	type modelRow struct {
		ID               int64  `json:"id"`
		ModelId          string `json:"model_id"`
		ModelName        string `json:"model_name"`
		Category         string `json:"category"`
		Status           string `json:"status"`
		MaxContextTokens int    `json:"max_context_tokens"`
		MaxOutputTokens  int    `json:"max_output_tokens"`
		Description      string `json:"description"`
		Capabilities     string `json:"capabilities"`
		CreatedAt        string `json:"created_at"`
	}

	var model *modelRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("id, model_id, model_name, category, status, max_context_tokens, max_output_tokens, description, capabilities, created_at").
		Scan(&model)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, common.ErrModelNotFound
	}

	// 校验租户权限（仅当 tenantID > 0 时）
	if tenantID > 0 {
		enabled, _, err := p.CheckTenantModelAccess(ctx, tenantID, modelName)
		if err != nil {
			return nil, err
		}
		if !enabled {
			return nil, common.ErrTenantModelNotEnabled
		}
	}

	// 解析 created_at 为 Unix timestamp
	var created int64
	if model.CreatedAt != "" {
		t, err := time.Parse(time.RFC3339, model.CreatedAt)
		if err == nil {
			created = t.Unix()
		}
	}

	return &common.ModelDetail{
		ID:               model.ModelId,
		Object:           "model",
		Created:          created,
		OwnedBy:          "platform",
		ModelName:        model.ModelName,
		Category:         model.Category,
		Status:           model.Status,
		MaxContextTokens: model.MaxContextTokens,
		MaxOutputTokens:  model.MaxOutputTokens,
		Description:      model.Description,
		Capabilities:     parseCapabilitiesJSON(model.Capabilities),
	}, nil
}

func parseCapabilitiesJSON(raw string) map[string]bool {
	if raw == "" || raw == "{}" {
		return nil
	}
	var caps map[string]bool
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return nil
	}
	return caps
}

// incrementConsecutiveFailure 递增连续失败计数
func incrementConsecutiveFailure(ctx context.Context, channelID int64) {
	g.DB().Exec(ctx,
		"UPDATE chn_health_scores SET consecutive_failures = consecutive_failures + 1, updated_at = ? WHERE channel_id = ? AND consecutive_failures < 10",
		time.Now(), channelID)
}

// modelInfoCached 模型信息缓存
type modelInfoCached struct {
	StandardName string
	Category     string
}

// InitHealthScore 初始化渠道健康度记录
func InitHealthScore(ctx context.Context, channelID int64) error {
	_, err := dao.ChnHealthScores.Ctx(ctx).Insert(do.ChnHealthScores{
		ChannelId:           channelID,
		SuccessRate:         100.00,
		LatencyMs:           0,
		StabilityScore:      100.00,
		ConsecutiveFailures: 0,
		HealthScore:         100.00,
		CalculatedAt:        gtime.Now(),
	})
	return err
}

// getChannelKey 获取渠道的 API Key（每渠道仅一个 Key）
func getChannelKey(ctx context.Context, channelID int64) (string, error) {
	type keyRow struct {
		ID             int64       `json:"id"`
		EncryptedKey   string      `json:"encrypted_key"`
		KeyType        string      `json:"key_type"`
		TokenExpiresAt *gtime.Time `json:"token_expires_at"`
	}

	var key *keyRow
	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", channelID).
		Where("status", "active").
		Fields("id, encrypted_key, key_type, token_expires_at").
		Scan(&key)
	if err != nil || key == nil {
		return "", common.ErrChannelUnavailable
	}

	// 更新最后使用时间（用于监控）
	dao.ChnChannelKeys.Ctx(ctx).
		Where("id", key.ID).
		Data(do.ChnChannelKeys{LastUsedAt: gtime.Now()}).
		Update()

	encKey := GetEncryptionKey()
	decrypted, err := uc.DecryptString(encKey, key.EncryptedKey)
	if err != nil {
		return "", err
	}

	// OAuth 按需刷新
	if key.KeyType == "oauth" && loauth.IsOAuthKeyData(decrypted) {
		var oauthData loauth.OAuthKeyData
		if err := json.Unmarshal([]byte(decrypted), &oauthData); err == nil {
			if oauthData.ExpiresAt > 0 && time.Now().Unix() > oauthData.ExpiresAt-300 {
				refreshed, refreshErr := refreshOAuthKey(ctx, key.ID, &oauthData, encKey)
				if refreshErr != nil {
					g.Log().Warningf(ctx, "[getChannelKey] OAuth refresh failed for key %d: %v", key.ID, refreshErr)
				} else if refreshed != "" {
					return refreshed, nil
				}
			}
			return decrypted, nil
		}
	}

	return decrypted, nil
}

// refreshOAuthKey 刷新 OAuth 令牌并更新数据库
func refreshOAuthKey(ctx context.Context, keyID int64, oauthData *loauth.OAuthKeyData, encKey []byte) (string, error) {
	var newToken *loauth.OAuthKeyData
	var err error

	switch oauthData.Platform {
	case "claude":
		newToken, err = loauth.ClaudeRefreshToken(oauthData.RefreshToken)
	case "openai":
		newToken, err = loauth.OpenAIRefreshToken(oauthData.RefreshToken)
	case "gemini":
		newToken, err = loauth.GeminiRefreshToken(oauthData.RefreshToken)
	default:
		return "", fmt.Errorf("unknown oauth platform: %s", oauthData.Platform)
	}
	if err != nil {
		return "", err
	}

	// 保留平台专属字段
	newToken.Platform = oauthData.Platform
	if newToken.RefreshToken == "" {
		newToken.RefreshToken = oauthData.RefreshToken
	}
	if newToken.OrgUUID == "" {
		newToken.OrgUUID = oauthData.OrgUUID
	}
	if newToken.AccountUUID == "" {
		newToken.AccountUUID = oauthData.AccountUUID
	}
	if newToken.EmailAddress == "" {
		newToken.EmailAddress = oauthData.EmailAddress
	}
	if newToken.AccountID == "" {
		newToken.AccountID = oauthData.AccountID
	}
	if newToken.UserID == "" {
		newToken.UserID = oauthData.UserID
	}
	if newToken.OrgID == "" {
		newToken.OrgID = oauthData.OrgID
	}
	if newToken.ProjectID == "" {
		newToken.ProjectID = oauthData.ProjectID
	}

	jsonData, err := json.Marshal(newToken)
	if err != nil {
		return "", err
	}

	encrypted, err := uc.EncryptString(encKey, string(jsonData))
	if err != nil {
		return "", err
	}

	expiresAt := gtime.NewFromTimeStamp(newToken.ExpiresAt)
	dao.ChnChannelKeys.Ctx(ctx).
		Where("id", keyID).
		Data(do.ChnChannelKeys{
			EncryptedKey:   encrypted,
			TokenExpiresAt: expiresAt,
		}).
		Update()

	return string(jsonData), nil
}

// GetModelDeprecationInfo 实现 DataProvider.GetModelDeprecationInfo
func (p *DataProviderImpl) GetModelDeprecationInfo(ctx context.Context, modelName string) (*common.DeprecationInfo, error) {
	type depRow struct {
		Status           string      `json:"status"`
		SunsetDate       *gtime.Time `json:"sunset_date"`
		ReplacementModel string      `json:"replacement_model"`
	}

	var row depRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("status, sunset_date, replacement_model").
		Scan(&row)
	if err != nil {
		if strings.Contains(err.Error(), "no rows in result set") {
			return nil, common.ErrModelNotFound
		}
		return nil, err
	}
	if row.Status == "" {
		return nil, common.ErrModelNotFound
	}
	if row.Status != "deprecated" {
		return nil, nil
	}

	info := &common.DeprecationInfo{
		Deprecated:       true,
		ReplacementModel: row.ReplacementModel,
	}
	if row.SunsetDate != nil {
		info.SunsetDate = row.SunsetDate.Format("Y-m-d")
	}
	return info, nil
}

// InvalidateModelCache 实现 DataProvider.InvalidateModelCache
func (p *DataProviderImpl) InvalidateModelCache(modelName string) {
	modelCache.Delete(context.Background(), modelName)
}

// memberModelCache 成员模型范围缓存（TTL 60s）
var memberModelCache = lcommon.NewCache("member_model", 60*time.Second)

// CheckMemberModelAccess 检查成员是否有权使用指定模型。
// 无 scope 记录表示不限制（向后兼容）。
func (p *DataProviderImpl) CheckMemberModelAccess(ctx context.Context, tenantID, userID int64, modelName string) (bool, error) {
	cacheKey := fmt.Sprintf("%d:%d", tenantID, userID)

	var cachedModelNames []string
	if memberModelCache.GetJSON(ctx, cacheKey, &cachedModelNames) {
		if len(cachedModelNames) == 0 {
			return true, nil
		}
		for _, name := range cachedModelNames {
			if name == modelName {
				return true, nil
			}
		}
		return false, nil
	}

	type scopeRow struct {
		ModelName string `json:"model_name"`
	}
	var rows []scopeRow
	err := g.DB().Model("tnt_member_model_scopes ms").Ctx(ctx).
		LeftJoin("mdl_models m ON ms.model_id = m.id").
		Where("ms.tenant_id", tenantID).
		Where("ms.user_id", userID).
		Fields("m.model_id as model_name").
		Scan(&rows)
	if err != nil {
		return false, err
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.ModelName)
	}

	memberModelCache.Set(ctx, cacheKey, names)

	if len(names) == 0 {
		return true, nil
	}
	for _, name := range names {
		if name == modelName {
			return true, nil
		}
	}
	return false, nil
}
