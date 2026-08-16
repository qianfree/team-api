package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/google/uuid"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"golang.org/x/sync/singleflight"

	"github.com/qianfree/team-api/internal/dispatchadapter"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	loauth "github.com/qianfree/team-api/internal/logic/common/oauth"
	uc "github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/relay/common"
)

// modelCache 模型信息缓存（TTL 600s）
var modelCache = lcommon.NewCache("model", 600*time.Second)

// modelCacheKeyPrefix 模型缓存 key 版本前缀：modelInfoCached 扩展弃用字段后用 v2 隔离旧格式缓存，
// 避免部署后 600s 内读到旧缓存（缺 status/sunset_date/replacement_model 字段）产生零值误判。
const modelCacheKeyPrefix = "v2:"

const channelKeyLastUsedInterval = time.Minute

var channelKeyUsage = newChannelKeyUsageTracker()

var oauthRefreshGroup singleflight.Group

type channelKeyUsageTracker struct {
	mu      sync.Mutex
	touched map[int64]time.Time
}

func newChannelKeyUsageTracker() *channelKeyUsageTracker {
	return &channelKeyUsageTracker{touched: make(map[int64]time.Time)}
}

func (t *channelKeyUsageTracker) claim(keyID int64, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ok := t.touched[keyID]; ok && now.Sub(last) < channelKeyLastUsedInterval {
		return false
	}
	t.touched[keyID] = now
	return true
}

func (t *channelKeyUsageTracker) release(keyID int64, claimedAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if last, ok := t.touched[keyID]; ok && last.Equal(claimedAt) {
		delete(t.touched, keyID)
	}
}

// tenantGroupModelCache 租户分组模型集合缓存（通过 lcommon.TenantGroupModelCache 共享）
// 实际缓存实例在 lcommon 包中，admin 和 relay 共用

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
		ID:                   info.ID,
		TenantID:             info.TenantID,
		UserID:               info.UserID,
		ProjectID:            info.ProjectID,
		Scope:                info.Scope,
		Status:               info.Status,
		RateLimitQps:         info.RateLimitQps,
		RateLimitConcurrency: info.RateLimitConcurrency,
		IpWhitelist:          info.IpWhitelist,
		TotalQuota:           info.TotalQuota,
		UsedQuota:            info.UsedQuota,
	}, nil
}

// CheckTenantModelAccess 实现 DataProvider.CheckTenantModelAccess
// 检查租户是否有权使用指定模型，返回是否启用和渠道范围
func (p *DataProviderImpl) CheckTenantModelAccess(ctx context.Context, tenantID int64, modelName string) (bool, []int64, error) {
	type accessRow struct {
		Enabled      bool   `json:"enabled"`
		ChannelScope string `json:"channel_scope"`
	}

	// 缓存结构体（与 accessRow 一致，用于缓存序列化）
	type cachedAccess struct {
		Enabled      bool   `json:"enabled"`
		ChannelScope string `json:"channel_scope"`
		IsNil        bool   `json:"is_nil"` // 标记数据库中无记录（需走分组权限）
	}

	cacheKey := fmt.Sprintf("%d:%s", tenantID, modelName)

	// 1. 尝试从缓存获取
	var cached cachedAccess
	if lcommon.TenantModelAccessCache.GetJSON(ctx, cacheKey, &cached) {
		// 缓存命中：无记录时走分组权限
		if cached.IsNil {
			return checkGroupModelAccess(ctx, tenantID, modelName)
		}
		// 有记录：解析渠道范围并返回
		var scope []int64
		if cached.ChannelScope != "" && cached.ChannelScope != "[]" && cached.ChannelScope != "null" {
			_ = json.Unmarshal([]byte(cached.ChannelScope), &scope)
		}
		return cached.Enabled, scope, nil
	}

	// 2. 缓存未命中：查询数据库
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

	// 3. 数据库无记录：缓存"无记录"标记，走分组权限
	if row == nil {
		lcommon.TenantModelAccessCache.Set(ctx, cacheKey, &cachedAccess{IsNil: true})
		return checkGroupModelAccess(ctx, tenantID, modelName)
	}

	// 4. 有记录：缓存结果
	lcommon.TenantModelAccessCache.Set(ctx, cacheKey, &cachedAccess{
		Enabled:      row.Enabled,
		ChannelScope: row.ChannelScope,
		IsNil:        false,
	})

	// 解析渠道范围 JSONB
	var scope []int64
	if row.ChannelScope != "" && row.ChannelScope != "[]" && row.ChannelScope != "null" {
		_ = json.Unmarshal([]byte(row.ChannelScope), &scope)
	}

	return row.Enabled, scope, nil
}

// checkGroupModelAccess 检查租户是否通过分组获得模型访问权限
func checkGroupModelAccess(ctx context.Context, tenantID int64, modelName string) (bool, []int64, error) {
	cacheKey := fmt.Sprintf("%d", tenantID)

	// 尝试从缓存获取
	cachedSet, found := lcommon.TenantGroupModelCache.Get(ctx, cacheKey)
	if found {
		if cachedSet == nil {
			return false, nil, nil
		}
		if modelSet, ok := cachedSet.(map[string]bool); ok {
			_, inGroup := modelSet[modelName]
			return inGroup, nil, nil
		}
	}

	// 缓存未命中：查询数据库
	groupCount, _ := dao.MdlTenantGroups.Ctx(ctx).
		Where("tenant_id", tenantID).
		Count()

	if groupCount == 0 {
		lcommon.TenantGroupModelCache.Set(ctx, cacheKey, nil)
		return false, nil, nil
	}

	// 查询租户所有活跃分组中的可用模型
	modelSet, err := queryTenantGroupModels(ctx, tenantID)
	if err != nil {
		return false, nil, err
	}

	// 缓存结果
	lcommon.TenantGroupModelCache.Set(ctx, cacheKey, modelSet)

	_, inGroup := modelSet[modelName]
	return inGroup, nil, nil
}

// queryTenantGroupModels 查询租户通过分组可访问的所有模型
func queryTenantGroupModels(ctx context.Context, tenantID int64) (map[string]bool, error) {
	var models []struct {
		ModelId string `json:"model_id"`
	}
	err := g.DB().Model("mdl_group_models gm").Ctx(ctx).
		InnerJoin("mdl_model_groups g ON gm.group_id = g.id").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		InnerJoin("mdl_tenant_groups tg ON tg.group_id = g.id").
		Where("tg.tenant_id", tenantID).
		Where("g.status", "active").
		Where("m.status", "active").
		Fields("m.model_id").
		Scan(&models)
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(models))
	for _, m := range models {
		result[m.ModelId] = true
	}
	return result, nil
}

// GetTenantGroupModelIDs 获取租户通过分组可访问的模型内部ID集合（用于 GetAvailableModels）
func GetTenantGroupModelIDs(ctx context.Context, tenantID int64) map[int64]bool {
	cacheKey := fmt.Sprintf("%d", tenantID)

	// 尝试从缓存获取 model_id string 集合
	cachedSet, found := lcommon.TenantGroupModelCache.Get(ctx, cacheKey)
	if found {
		if cachedSet == nil {
			return nil
		}
		// 缓存中存的是 map[string]bool，需要转换为 map[int64]bool
		// 这里直接查数据库获取 int64 版本
	}

	// 查询模型内部 ID
	var models []struct {
		Id int64 `json:"id"`
	}
	err := g.DB().Model("mdl_group_models gm").Ctx(ctx).
		InnerJoin("mdl_model_groups g ON gm.group_id = g.id").
		InnerJoin("mdl_models m ON gm.model_id = m.id").
		InnerJoin("mdl_tenant_groups tg ON tg.group_id = g.id").
		Where("tg.tenant_id", tenantID).
		Where("g.status", "active").
		Where("m.status", "active").
		Fields("m.id").
		Scan(&models)
	if err != nil || len(models) == 0 {
		return nil
	}

	result := make(map[int64]bool, len(models))
	for _, m := range models {
		result[m.Id] = true
	}
	return result
}

// GetModelMapping 实现 DataProvider.GetModelMapping
func (p *DataProviderImpl) GetModelMapping(ctx context.Context, modelName string) (string, string, error) {
	info, err := loadModelCached(ctx, modelName)
	if err != nil {
		return "", "", err
	}
	if info == nil || info.Status == "offline" {
		return "", "", common.ErrModelNotFound
	}
	// deprecated 模型放行，弃用信息由 GetModelDeprecationInfo 获取
	return info.StandardName, info.Category, nil
}

// loadModelCached 读取模型信息：优先 600s 缓存，未命中时一次查询 mdl_models 的全部所需字段
// （mapping + 弃用信息）并回填缓存。GetModelMapping 与 GetModelDeprecationInfo 共用，
// 避免同一请求对同一模型发两条 SQL。返回 nil 表示模型中不存在；不缓存负向结果。
func loadModelCached(ctx context.Context, modelName string) (*modelInfoCached, error) {
	cacheKey := modelCacheKeyPrefix + modelName
	var cached modelInfoCached
	if modelCache.GetJSON(ctx, cacheKey, &cached) {
		return &cached, nil
	}

	type modelRow struct {
		ModelId          string      `json:"model_id"`
		Category         string      `json:"category"`
		Status           string      `json:"status"`
		SunsetDate       *gtime.Time `json:"sunset_date"`
		ReplacementModel string      `json:"replacement_model"`
	}

	var model *modelRow
	err := dao.MdlModels.Ctx(ctx).
		Where("model_id", modelName).
		Fields("model_id, category, status, sunset_date, replacement_model").
		Scan(&model)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}

	sunset := ""
	if model.SunsetDate != nil {
		sunset = model.SunsetDate.Format("Y-m-d")
	}
	info := &modelInfoCached{
		StandardName:     model.ModelId,
		Category:         model.Category,
		Status:           model.Status,
		SunsetDate:       sunset,
		ReplacementModel: model.ReplacementModel,
	}
	modelCache.Set(ctx, cacheKey, info)
	return info, nil
}

// RecordUsage 实现 DataProvider.RecordUsage
func (p *DataProviderImpl) RecordUsage(ctx context.Context, record *common.UsageRecord) {
	lcommon.DefaultUsageLogWriter.Submit(buildUsageLogDO(record))

	// 实时 RPM/TPM 计数（Redis 滑动窗口桶，内部异步、best-effort）
	tokens := record.TotalTokens
	if tokens == 0 {
		tokens = record.PromptTokens + record.CompletionTokens
	}
	lcommon.RecordRealtimeMetrics(ctx, record.TenantID, int64(tokens))

	// 模型当日性能热桶（Redis 小时桶，内部异步、best-effort）：供模型性能页展示当天实时数据，
	// 次日由 usage_daily_aggregate 聚合进 bil_usage_daily 后不再依赖。
	lcommon.RecordModelPerfMetrics(ctx, lcommon.ModelPerfRecord{
		Model:        record.ModelName,
		Success:      record.Success,
		LatencyMs:    record.LatencyMs,
		FirstTokenMs: record.FirstTokenMs,
		InputTokens:  record.PromptTokens,
		OutputTokens: record.CompletionTokens,
		CostUSD:      record.TotalCost,
		// 缓存 token 透传给热桶（命中请求判定由热桶模块从 read>0 推导）
		CacheCreationTokens: record.CacheCreationTokens,
		CacheReadTokens:     record.CacheReadTokens,
	})
}

// sanitizeUTF8 清洗字符串字段：将非法 UTF-8 字节替换为 U+FFFD 替换符。
// 上游错误文本/请求头/回显模型名等可能夹带 0xa0 等单字节脏数据，PostgreSQL 在协议层
// 会以 invalid byte sequence 拒绝整批插入，必须在落库前清洗保证字符串合法。
func sanitizeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	return strings.ToValidUTF8(s, "�")
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
		ModelName:    sanitizeUTF8(record.ModelName),
		RequestId:    sanitizeUTF8(record.RequestID),
		RelayMode:    record.RelayMode,
		InputTokens:  record.PromptTokens,
		OutputTokens: record.CompletionTokens,
		TotalCost:    record.TotalCost,
		LatencyMs:    int(record.LatencyMs),
		Status:       sanitizeUTF8(record.Status),
		ErrorMessage: sanitizeUTF8(record.ErrorMessage),
		ClientIp:     sanitizeUTF8(record.ClientIP),

		InputCost:         record.InputCost,
		OutputCost:        record.OutputCost,
		CacheCreationCost: record.CacheCreationCost,
		CacheReadCost:     record.CacheReadCost,
		ActualCost:        record.ActualCost,
		Currency:          sanitizeUTF8(record.Currency),
		PreDeductAmount:   record.PreDeductAmount,
		RefundAmount:      record.RefundAmount,
		SupplementAmount:  record.SupplementAmount,

		CacheCreationTokens: record.CacheCreationTokens,
		CacheReadTokens:     record.CacheReadTokens,

		AudioInputTokens:  record.AudioInputTokens,
		AudioOutputTokens: record.AudioOutputTokens,
		ImageOutputTokens: record.ImageOutputTokens,
		ReasoningTokens:   record.ReasoningTokens,

		RequestedModel: sanitizeUTF8(record.RequestedModel),
		UpstreamModel:  sanitizeUTF8(record.UpstreamModel),

		RequestType:     requestType,
		UserAgent:       sanitizeUTF8(record.UserAgent),
		FirstTokenMs:    record.FirstTokenMs,
		ServiceTier:     sanitizeUTF8(record.ServiceTier),
		ReasoningEffort: sanitizeUTF8(record.ReasoningEffort),
		InboundEndpoint: sanitizeUTF8(record.InboundEndpoint),

		ChannelName: sanitizeUTF8(record.ChannelName),
		ChannelType: record.ChannelType,

		BillingMode:    sanitizeUTF8(record.BillingMode),
		BillingSource:  sanitizeUTF8(record.BillingSource),
		RateMultiplier: record.RateMultiplier,
		RetryIndex:     record.RetryIndex,

		StreamEndReason: sanitizeUTF8(record.StreamEndReason),

		ImageCount: record.ImageCount,
		ImageSize:  sanitizeUTF8(record.ImageSize),

		BillingSnapshot: jsonNullIfEmpty(sanitizeUTF8(record.BillingSnapshot)),
		BillingSummary:  sanitizeUTF8(record.BillingSummary),
		TaskId:          sanitizeUTF8(record.TaskID),
	}
}

// maxAuditBodyLen 未配置独立审计库时，审计日志请求体/响应体/任务结果的字节截断上限（2 KiB）。
// 落库前由 truncateBody 统一截断；流式响应保留首部 + 末尾若干 SSE 消息。
// 配置了独立审计库（database.audit）时 auditBodyMaxLen 返回 0，不截断、按审计规则完整记录。
const maxAuditBodyLen = 2048

// auditBodyMaxLen 返回审计请求体/响应体/任务结果的落库截断上限。
// 配置了独立审计库时返回 0（truncateBody 视为不截断），此时有充足存储空间按审计规则完整记录；
// 未配置时返回 maxAuditBodyLen，防止与业务数据共用主库导致库膨胀。
func auditBodyMaxLen() int {
	if lcommon.IsAuditDBConfigured() {
		return 0
	}
	return maxAuditBodyLen
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

		// 截断过长的内容：配置了独立审计库时完整记录（auditBodyMaxLen=0 不截断），
		// 否则截断到 2 KiB，防止与业务数据共用主库导致库膨胀。
		maxBodyLen := auditBodyMaxLen()
		sysReq, sysResp = truncateBody(sysReq, maxBodyLen), truncateBody(sysResp, maxBodyLen)
		tntReq, tntResp = truncateBody(tntReq, maxBodyLen), truncateBody(tntResp, maxBodyLen)

		// 仅审计级别为 all 时记录请求头和响应头
		requestHeadersJSON := "null"
		responseHeadersJSON := "null"
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
		forwardingTraceJSON := "null"
		if record.ForwardingTrace != nil {
			if b, err := json.Marshal(record.ForwardingTrace); err == nil {
				forwardingTraceJSON = string(b)
			}
		}

		insertData := do.AudRequestLogs{
			TenantId:           record.TenantID,
			UserId:             record.UserID,
			ApiKeyId:           record.ApiKeyID,
			ProjectId:          record.ProjectID,
			RequestId:          record.RequestID,
			Method:             record.Method,
			Path:               record.Path,
			QueryParams:        record.QueryParams,
			StatusCode:         record.StatusCode,
			ClientIp:           record.ClientIP,
			UserAgent:          record.UserAgent,
			RequestBody:        sysReq,
			ResponseBody:       sysResp,
			TenantRequestBody:  tntReq,
			TenantResponseBody: tntResp,
			LatencyMs:          record.LatencyMs,
			FirstTokenMs:       record.FirstTokenMs,
			AuditLevel:         globalLevel,
			TenantAuditLevel:   tenantLevel,
			RequestHeaders:     requestHeadersJSON,
			ResponseHeaders:    responseHeadersJSON,
			ForwardingTrace:    forwardingTraceJSON,
		}

		// 异步任务字段
		if record.TaskID != "" {
			insertData.TaskId = record.TaskID
		}
		if record.TaskStatus != "" {
			insertData.TaskStatus = record.TaskStatus
		}

		// 投递到批量写入器：审计日志与请求 1:1，走 channel 批量落库（64 条/批或 1s 一次），
		// 避免每请求一条单条 INSERT 打爆审计库。队列满时写入器按 OverflowSyncFallback 降级同步写，
		// 审计数据不允许静默丢弃。
		if lcommon.DefaultAuditLogWriter == nil {
			// 写入器未初始化（单元测试等场景）：降级为同步单条写入，保证审计不丢
			_, insertErr := lcommon.AuditModelCtx(bgCtx, "aud_request_logs").Data(insertData).Insert()
			if insertErr != nil {
				g.Log().Errorf(bgCtx,
					"record audit log failed: request_id=%s tenant_id=%d api_key_id=%d path=%s status=%d err=%v",
					record.RequestID, record.TenantID, record.ApiKeyID, record.Path, record.StatusCode, insertErr)
			}
			return
		}
		lcommon.DefaultAuditLogWriter.Submit(insertData)
	}()
}

// UpdateTaskAudit implements DataProvider.UpdateTaskAudit
// 异步任务完成时更新提交阶段写入的审计记录
func (p *DataProviderImpl) UpdateTaskAudit(ctx context.Context, record *common.AuditRecord) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Errorf(context.Background(),
					"update task audit panic: task_id=%s panic=%v",
					record.TaskID, r)
			}
		}()

		if record.TaskID == "" {
			return
		}

		bgCtx := context.Background()
		globalLevel, tenantLevel := lcommon.GetAuditLevels(bgCtx, record.TenantID)

		// 按审计级别处理任务结果
		taskResult := record.TaskResult
		if globalLevel == lcommon.AuditLevelNone && tenantLevel == lcommon.AuditLevelNone {
			taskResult = ""
		} else if globalLevel == lcommon.AuditLevelMasked {
			taskResult = lcommon.MaskSensitiveData(taskResult)
		}
		taskResult = truncateBody(taskResult, auditBodyMaxLen())

		// 上游响应头仅在 full 级别记录
		upstreamHeadersJSON := "null"
		if globalLevel == lcommon.AuditLevelFull && record.TaskUpstreamHeaders != nil {
			b, _ := json.Marshal(record.TaskUpstreamHeaders)
			upstreamHeadersJSON = string(b)
		}

		updateData := do.AudRequestLogs{
			TaskStatus:          record.TaskStatus,
			TaskResult:          taskResult,
			TaskUpstreamHeaders: upstreamHeadersJSON,
		}
		if record.TaskCompletedAt != nil {
			updateData.TaskCompletedAt = gtime.NewFromTime(*record.TaskCompletedAt)
		}
		if record.LatencyMs > 0 {
			updateData.LatencyMs = record.LatencyMs
		}

		// 提交阶段的审计 INSERT 是异步的（RecordAudit 内 go func 落库）。对 sync_image 这类
		// 提交后立即入队、worker 可能极快出终态的任务，完成回调有可能抢在 INSERT 之前到达，
		// 导致按 task_id 更新命中 0 行、终态（SUCCESS/FAILURE）丢失，审计里的任务状态一直停在
		// 「已提交」。这里在命中 0 行时短暂重试，兜住这个竞态窗口。
		// 审计关闭时提交阶段本就不写行，无需重试，直接返回避免空转。
		auditDisabled := globalLevel == lcommon.AuditLevelNone && tenantLevel == lcommon.AuditLevelNone
		// 提交阶段的审计 INSERT 已走批量通道（64 条/批或 1s flush），落库延迟比单条写更长，
		// 重试窗口需覆盖批量 flush 周期：10 次线性退避最迟约 6.75s，足以等行落库。
		const maxAttempts = 10
		for attempt := 1; ; attempt++ {
			res, err := lcommon.AuditModelCtx(bgCtx, "aud_request_logs").
				Where("task_id", record.TaskID).
				Data(updateData).
				Update()
			if err != nil {
				g.Log().Errorf(bgCtx,
					"update task audit failed: task_id=%s attempt=%d err=%v",
					record.TaskID, attempt, err)
				return
			}
			if affected, _ := res.RowsAffected(); affected > 0 {
				return
			}
			if auditDisabled || attempt >= maxAttempts {
				if !auditDisabled {
					g.Log().Warningf(bgCtx,
						"update task audit affected 0 rows after %d attempts: task_id=%s (submit audit not persisted yet?)",
						maxAttempts, record.TaskID)
				}
				return
			}
			// 线性退避：150ms、300ms……最迟约 6.75s，覆盖批量写入的 flush 周期。
			time.Sleep(time.Duration(attempt) * 150 * time.Millisecond)
		}
	}()
}

func truncateBody(s string, maxLen int) string {
	// maxLen <= 0 表示不截断（配置了独立审计库时完整记录）
	if maxLen <= 0 || len(s) <= maxLen {
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

// GetAvailableModels 实现 DataProvider.GetAvailableModels
// 如果 tenantID > 0，返回该租户有权使用的模型列表
// 如果 apiKeyID > 0，进一步按 API Key 的模型范围过滤
func (p *DataProviderImpl) GetAvailableModels(ctx context.Context, tenantID, apiKeyID, userID int64) ([]common.ModelInfo, error) {
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

	fieldsNoAlias := "model_id, model_name, category, status, max_context_tokens, max_output_tokens, capabilities"

	if tenantID > 0 {
		// 获取显式分配的模型
		tenantModelCount, _ := dao.MdlTenantModels.Ctx(ctx).
			Where("tenant_id", tenantID).
			Count()

		// 获取分组分配的模型
		groupModelIDs := GetTenantGroupModelIDs(ctx, tenantID)

		hasExplicit := tenantModelCount > 0
		hasGroup := len(groupModelIDs) > 0

		if !hasExplicit && !hasGroup {
			// 无分配记录且无分组：返回空列表
			models = make([]modelRow, 0)
		} else {
			// 构建合并的模型 ID 集合
			allModelIDs := make([]int64, 0)

			if hasExplicit {
				var explicitIDs []struct {
					ModelID int64 `json:"model_id"`
				}
				if err := dao.MdlTenantModels.Ctx(ctx).
					Where("tenant_id", tenantID).Where("enabled", true).
					Fields("model_id").Scan(&explicitIDs); err != nil {
					g.Log().Warningf(ctx, "[DataProvider] GetAvailableModels scan tenant models failed: tenantID=%d, err=%v", tenantID, err)
				}
				for _, id := range explicitIDs {
					allModelIDs = append(allModelIDs, id.ModelID)
				}
			}

			for id := range groupModelIDs {
				allModelIDs = append(allModelIDs, id)
			}

			if len(allModelIDs) > 0 {
				err = dao.MdlModels.Ctx(ctx).
					WhereIn("id", allModelIDs).
					Where("status", "active").
					Fields(fieldsNoAlias).
					OrderAsc("category").
					OrderAsc("model_id").
					Scan(&models)
			}
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

	// 按成员模型范围过滤
	if userID > 0 {
		allowedNames, err := GetMemberAllowedModelNames(ctx, tenantID, userID)
		if err != nil {
			return nil, err
		}
		if allowedNames != nil {
			if len(allowedNames) == 0 {
				models = nil
			} else {
				allowed := make(map[string]bool, len(allowedNames))
				for _, n := range allowedNames {
					allowed[n] = true
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
// 获取单个模型的详细信息，同时校验租户 / API Key / 成员的模型范围权限
func (p *DataProviderImpl) GetModelDetail(ctx context.Context, tenantID, apiKeyID, userID int64, modelName string) (*common.ModelDetail, error) {
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

	// 校验 API Key 模型范围（与 /v1/models 列表口径一致，仅当 apiKeyID > 0 时）
	if apiKeyID > 0 {
		if allowed, err := p.CheckApiKeyModelAccess(ctx, apiKeyID, modelName); err != nil {
			return nil, err
		} else if !allowed {
			return nil, common.ErrApiKeyModelNotAllowed
		}
	}

	// 校验成员模型范围（与 /v1/models 列表口径一致，仅当 userID > 0 时）
	if userID > 0 {
		if allowed, err := p.CheckMemberModelAccess(ctx, tenantID, userID, modelName); err != nil {
			return nil, err
		} else if !allowed {
			return nil, common.ErrMemberModelNotAllowed
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
// modelInfoCached 模型信息缓存（mapping 与弃用信息一次查询共用）
type modelInfoCached struct {
	StandardName     string
	Category         string
	Status           string
	SunsetDate       string // 格式 "2006-01-02"，空表示未设置
	ReplacementModel string
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

// MaterializeSelection 由新调度引擎的决策构造 ChannelSelection（阶段 3）：
// 转发元数据来自目录内存快照（零 DB），渠道 Key 按 keyID 解密（keyID=0 回退取首个 active Key）。
func (p *DataProviderImpl) MaterializeSelection(ctx context.Context, channelID, keyID int64, modelName string) (*common.ChannelSelection, error) {
	// 优先检查上下文：客户端已断开时直接返回 ctx.Err()，而非 ErrChannelUnavailable，
	// 让上层能区分"客户端断开"与"真实渠道无可用 Key"，避免误伤渠道健康评分
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	catalog := dispatchadapter.CatalogInstance()
	if catalog == nil {
		return nil, common.ErrChannelUnavailable
	}
	meta, ok := catalog.ForwardMeta(channelID, modelName)
	if !ok {
		return nil, common.ErrChannelUnavailable
	}

	var apiKey string
	var err error
	if keyID > 0 {
		apiKey, err = getChannelKeyByID(ctx, keyID)
	} else {
		apiKey, err = getChannelKey(ctx, channelID)
	}
	if err != nil || apiKey == "" {
		// Key 查询失败：再次检查 ctx，区分"客户端断开导致查询中断"与"真实无可用 Key"
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, common.ErrChannelUnavailable
	}

	return &common.ChannelSelection{
		ChannelID:         meta.ChannelID,
		ChannelType:       meta.ChannelType,
		ChannelName:       meta.ChannelName,
		BaseURL:           meta.BaseURL,
		ApiKey:            apiKey,
		KeyID:             keyID,
		UpstreamModelName: meta.UpstreamModel,
		IsModelMapped:     meta.IsModelMapped,
		MaxConcurrency:    meta.MaxConcurrency,
		Settings:          ParseChannelSettings(meta.Settings),
		SupportsResponses: meta.SupportsResponses,
		ChatViaResponses:  meta.ChatViaResponses,
	}, nil
}

// getChannelKey 获取渠道的 API Key（每渠道仅一个 Key）
func getChannelKey(ctx context.Context, channelID int64) (string, error) {
	var key *channelKeyRow
	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", channelID).
		Where("status", "active").
		Fields("id, encrypted_key, key_type, token_expires_at").
		Scan(&key)
	if err != nil || key == nil {
		return "", common.ErrChannelUnavailable
	}
	return decryptChannelKey(ctx, key)
}

// channelKeyRow 渠道 Key 查询行（getChannelKey / getChannelKeyByID / getChannelKeyExcluding 共用）。
type channelKeyRow struct {
	ID             int64       `json:"id"`
	EncryptedKey   string      `json:"encrypted_key"`
	KeyType        string      `json:"key_type"`
	TokenExpiresAt *gtime.Time `json:"token_expires_at"`
}

// getChannelKeyByID 按 KeyID 取渠道 Key 明文（修订 R1 凭证轮换链路：
// 调度决策已指定 KeyID 时使用），复用解密与 OAuth 按需刷新逻辑。
func getChannelKeyByID(ctx context.Context, keyID int64) (string, error) {
	var key *channelKeyRow
	err := dao.ChnChannelKeys.Ctx(ctx).
		Where("id", keyID).
		Where("status", "active").
		Fields("id, encrypted_key, key_type, token_expires_at").
		Scan(&key)
	if err != nil || key == nil {
		return "", common.ErrChannelUnavailable
	}
	return decryptChannelKey(ctx, key)
}

// decryptChannelKey 解密渠道 Key 并做 OAuth 按需刷新（共享逻辑，行为与原 getChannelKey 一致）。
func decryptChannelKey(ctx context.Context, key *channelKeyRow) (string, error) {
	encKey := GetEncryptionKey()
	decrypted, err := uc.DecryptString(encKey, key.EncryptedKey)
	if err != nil {
		return "", err
	}
	touchChannelKeyLastUsed(ctx, key.ID)

	// OAuth 按需刷新
	if key.KeyType == "oauth" && loauth.IsOAuthKeyData(decrypted) {
		var oauthData loauth.OAuthKeyData
		if err := json.Unmarshal([]byte(decrypted), &oauthData); err == nil {
			if oauthData.ExpiresAt > 0 && time.Now().Unix() > oauthData.ExpiresAt-300 {
				refreshed, refreshErr := refreshOAuthKey(ctx, key.ID, encKey)
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

func touchChannelKeyLastUsed(ctx context.Context, keyID int64) {
	now := time.Now()
	if !channelKeyUsage.claim(keyID, now) {
		return
	}

	_, err := dao.ChnChannelKeys.Ctx(ctx).
		Where("id", keyID).
		Where("last_used_at IS NULL OR last_used_at < ?", now.Add(-channelKeyLastUsedInterval)).
		Data(do.ChnChannelKeys{LastUsedAt: gtime.New(now)}).
		Update()
	if err != nil {
		channelKeyUsage.release(keyID, now)
		g.Log().Debugf(ctx, "[DataProvider] update channel key LastUsedAt failed: keyID=%d, err=%v", keyID, err)
	}
}

// refreshOAuthKey 刷新 OAuth 令牌并更新数据库
func refreshOAuthKey(ctx context.Context, keyID int64, encKey []byte) (string, error) {
	value, err, _ := oauthRefreshGroup.Do(strconv.FormatInt(keyID, 10), func() (any, error) {
		release, lockErr := acquireOAuthRefreshLock(ctx, keyID)
		if lockErr != nil {
			return "", lockErr
		}
		defer release()
		return refreshOAuthKeyLocked(ctx, keyID, encKey)
	})
	if err != nil {
		return "", err
	}
	return value.(string), nil
}

func acquireOAuthRefreshLock(ctx context.Context, keyID int64) (func(), error) {
	lockKey := fmt.Sprintf("oauth:refresh:lock:%d", keyID)
	lockValue := uuid.NewString()
	timeout := time.NewTimer(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		result, err := g.Redis().Do(ctx, "SET", lockKey, lockValue, "NX", "EX", 60)
		if err != nil {
			// Redis 故障时仍由 singleflight 提供本进程保护，避免刷新功能完全不可用。
			g.Log().Warningf(ctx, "[OAuthRefresh] redis lock unavailable for key %d: %v", keyID, err)
			return func() {}, nil
		}
		if !result.IsNil() {
			return func() {
				const releaseLua = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0`
				_, _ = g.Redis().Do(context.Background(), "EVAL", releaseLua, 1, lockKey, lockValue)
			}, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout.C:
			return nil, fmt.Errorf("timed out waiting for OAuth refresh lock for key %d", keyID)
		case <-ticker.C:
		}
	}
}

func refreshOAuthKeyLocked(ctx context.Context, keyID int64, encKey []byte) (string, error) {
	var row *struct {
		EncryptedKey string `json:"encrypted_key"`
	}
	if err := dao.ChnChannelKeys.Ctx(ctx).
		Where("id", keyID).
		Where("status", "active").
		Fields("encrypted_key").
		Scan(&row); err != nil {
		return "", err
	}
	if row == nil {
		return "", common.ErrChannelUnavailable
	}

	decrypted, err := uc.DecryptString(encKey, row.EncryptedKey)
	if err != nil {
		return "", err
	}
	var oauthData loauth.OAuthKeyData
	if err = json.Unmarshal([]byte(decrypted), &oauthData); err != nil {
		return "", err
	}
	if oauthData.ExpiresAt <= 0 || time.Now().Unix() <= oauthData.ExpiresAt-300 {
		return decrypted, nil
	}

	var newToken *loauth.OAuthKeyData

	switch oauthData.Platform {
	case "claude":
		newToken, err = loauth.ClaudeRefreshToken(oauthData.RefreshToken)
	case "openai", "codex":
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
	_, err = dao.ChnChannelKeys.Ctx(ctx).
		Where("id", keyID).
		Data(do.ChnChannelKeys{
			EncryptedKey:   encrypted,
			TokenExpiresAt: expiresAt,
		}).Update()
	if err != nil {
		return "", err
	}

	return string(jsonData), nil
}

// GetModelDeprecationInfo 实现 DataProvider.GetModelDeprecationInfo
// 复用 loadModelCached 的单次查询结果（含 600s 缓存），不再独立查库。
func (p *DataProviderImpl) GetModelDeprecationInfo(ctx context.Context, modelName string) (*common.DeprecationInfo, error) {
	info, err := loadModelCached(ctx, modelName)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, common.ErrModelNotFound
	}
	if info.Status != "deprecated" {
		return nil, nil
	}

	dep := &common.DeprecationInfo{
		Deprecated:       true,
		ReplacementModel: info.ReplacementModel,
	}
	if info.SunsetDate != "" {
		dep.SunsetDate = info.SunsetDate
	}
	return dep, nil
}

// InvalidateModelCache 实现 DataProvider.InvalidateModelCache
func (p *DataProviderImpl) InvalidateModelCache(modelName string) {
	modelCache.Delete(context.Background(), modelCacheKeyPrefix+modelName)
}

// ClearTenantModelAccessCache 清除租户的模型访问权限缓存
// 在管理后台修改租户模型配置（增删改）后调用，避免高并发下读到陈旧缓存
func ClearTenantModelAccessCache(ctx context.Context, tenantID int64) {
	// 使用 pattern 删除该租户的所有模型访问缓存
	pattern := fmt.Sprintf("%d:*", tenantID)
	lcommon.TenantModelAccessCache.DeleteByPattern(ctx, pattern)
	g.Log().Infof(ctx, "[Cache] Cleared tenant model access cache for tenant_id=%d", tenantID)
}

// memberModelCache 成员模型范围缓存（TTL 60s）。
// 这是该缓存唯一的生产者（Get/Set），tenant 与 open 业务包通过
// InvalidateMemberModelScopeCache 显式失效本缓存（写操作后调用），
// 共享 "member_model" 前缀是故意的跨包失效机制，并非命名冲突——
// 详见 InvalidateMemberModelScopeCache。
var memberModelCache = lcommon.NewCache("member_model", 60*time.Second)

// InvalidateMemberModelScopeCache 失效指定成员的模型范围缓存。
// 供 tenant（管理后台改成员模型范围）与 open（开放平台改成员模型范围）在写库后调用，
// 是 memberModelCache 唯一的跨包失效入口，避免在三处各自重复 "member_model" 前缀字面量。
func InvalidateMemberModelScopeCache(ctx context.Context, tenantID, userID int64) {
	memberModelCache.Delete(ctx, fmt.Sprintf("%d:%d", tenantID, userID))
}

// GetMemberAllowedModelNames 获取成员允许使用的 model_id 列表。
// 返回 nil 表示不限制（向后兼容），返回空切片表示禁止所有模型（哨兵），返回非空切片表示允许的模型集合。
func GetMemberAllowedModelNames(ctx context.Context, tenantID, userID int64) ([]string, error) {
	if userID <= 0 || tenantID <= 0 {
		return nil, nil
	}

	cacheKey := fmt.Sprintf("%d:%d", tenantID, userID)

	var cachedNames []string
	if memberModelCache.GetJSON(ctx, cacheKey, &cachedNames) {
		return cachedNames, nil
	}

	type scopeRow struct {
		ModelID   int64  `json:"model_id"`
		ModelName string `json:"model_name"`
	}
	var rows []scopeRow
	err := g.DB().Model("tnt_member_model_scopes ms").Ctx(ctx).
		LeftJoin("mdl_models m ON ms.model_id = m.id").
		Where("ms.tenant_id", tenantID).
		Where("ms.user_id", userID).
		Fields("ms.model_id, m.model_id as model_name").
		Scan(&rows)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, nil
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ModelID == -1 {
			memberModelCache.Set(ctx, cacheKey, []string{})
			return []string{}, nil
		}
		if r.ModelName != "" {
			names = append(names, r.ModelName)
		}
	}

	memberModelCache.Set(ctx, cacheKey, names)
	return names, nil
}

// CheckMemberModelAccess 检查成员是否有权使用指定模型。
// 无 scope 记录表示不限制（向后兼容）。
// 哨兵 model_id=-1 表示该成员被限制使用所有模型。
func (p *DataProviderImpl) CheckMemberModelAccess(ctx context.Context, tenantID, userID int64, modelName string) (bool, error) {
	cacheKey := fmt.Sprintf("%d:%d", tenantID, userID)

	var cachedModelNames []string
	if memberModelCache.GetJSON(ctx, cacheKey, &cachedModelNames) {
		if len(cachedModelNames) == 0 {
			return true, nil
		}
		// 哨兵值：表示该成员被限制使用所有模型
		for _, name := range cachedModelNames {
			if name == "__none__" {
				return false, nil
			}
		}
		for _, name := range cachedModelNames {
			if name == modelName {
				return true, nil
			}
		}
		return false, nil
	}

	type scopeRow struct {
		ModelID   int64  `json:"model_id"`
		ModelName string `json:"model_name"`
	}
	var rows []scopeRow
	err := g.DB().Model("tnt_member_model_scopes ms").Ctx(ctx).
		LeftJoin("mdl_models m ON ms.model_id = m.id").
		Where("ms.tenant_id", tenantID).
		Where("ms.user_id", userID).
		Fields("ms.model_id, m.model_id as model_name").
		Scan(&rows)
	if err != nil {
		return false, err
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ModelID == -1 {
			names = append(names, "__none__")
		} else if r.ModelName != "" {
			names = append(names, r.ModelName)
		}
	}

	memberModelCache.Set(ctx, cacheKey, names)

	if len(names) == 0 {
		return true, nil
	}
	// 哨兵值：表示该成员被限制使用所有模型
	for _, name := range names {
		if name == "__none__" {
			return false, nil
		}
	}
	for _, name := range names {
		if name == modelName {
			return true, nil
		}
	}
	return false, nil
}

// InvalidateMemberModelCache 实现 DataProvider.InvalidateMemberModelCache
func (p *DataProviderImpl) InvalidateMemberModelCache(ctx context.Context, tenantID, userID int64) {
	cacheKey := fmt.Sprintf("%d:%d", tenantID, userID)
	memberModelCache.Delete(ctx, cacheKey)
}

// apiKeyModelCache API Key 模型范围缓存（TTL 60s）
var apiKeyModelCache = lcommon.NewCache("apikey_model", 60*time.Second)

// CheckApiKeyModelAccess 检查 API Key 是否有权使用指定模型。
// 无 scope 记录表示不限制（向后兼容）。
func (p *DataProviderImpl) CheckApiKeyModelAccess(ctx context.Context, apiKeyID int64, modelName string) (bool, error) {
	cacheKey := strconv.FormatInt(apiKeyID, 10)

	var cachedModelNames []string
	if apiKeyModelCache.GetJSON(ctx, cacheKey, &cachedModelNames) {
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
	err := dao.ApiKeyModelScopes.Ctx(ctx).
		Where("api_key_id", apiKeyID).
		Fields("model_name").
		Scan(&rows)
	if err != nil {
		return false, err
	}

	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.ModelName)
	}

	apiKeyModelCache.Set(ctx, cacheKey, names)

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

// InvalidateApiKeyModelCache 清除指定 API Key 的模型权限缓存
func (p *DataProviderImpl) InvalidateApiKeyModelCache(ctx context.Context, apiKeyID int64) {
	cacheKey := strconv.FormatInt(apiKeyID, 10)
	apiKeyModelCache.Delete(ctx, cacheKey)
}
