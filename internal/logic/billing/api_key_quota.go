package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
)

const (
	ApiKeyQuotaRedisKeyPrefix = "api_key_quota:"
	ApiKeyQuotaCacheTTL       = 60 // seconds
)

// apiKeyQuotaInfo API Key 额度缓存结构
type apiKeyQuotaInfo struct {
	TotalQuota float64 `json:"total_quota"`
	UsedQuota  float64 `json:"used_quota"`
}

func apiKeyQuotaRedisKey(apiKeyID int64) string {
	return fmt.Sprintf("%s%d", ApiKeyQuotaRedisKeyPrefix, apiKeyID)
}

// CheckApiKeyQuota checks whether an API key has enough remaining quota.
// total_quota <= 0 means unlimited.
// 与 CheckMemberQuota 一致走 Redis 缓存（TTL 60s）：同一请求内预扣前会检查两次
// （0 元快速闸门 + 按实际预扣额复查），缓存让第二次命中 Redis，避免每请求两次裸查 api_keys。
// 额度是控制线允许最终一致：used_quota 的短暂滞后由 IncrApiKeyQuotaUsed 落库后失效兜底；
// 管理端调整 total_quota 后最多延迟 TTL（60s）生效，与成员额度缓存行为一致。
func CheckApiKeyQuota(ctx context.Context, apiKeyID int64, preDeductAmount float64) error {
	if apiKeyID <= 0 {
		return nil
	}

	info, err := loadApiKeyQuota(ctx, apiKeyID)
	if err != nil {
		// 客户端断开导致的取消不属于 DB 故障，不刷告警：请求随后会在预扣/调度处 fail-fast 终止
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		g.Log().Warningf(ctx, "api_key_quota: load failed apiKey=%d: %v, skipping check", apiKeyID, err)
		return nil
	}

	if info.TotalQuota <= 0 {
		return nil
	}
	if info.UsedQuota+preDeductAmount > info.TotalQuota {
		return gerror.New("API key quota exceeded")
	}

	return nil
}

// loadApiKeyQuota 读取 API Key 额度：优先 Redis 缓存，未命中时查询数据库并回填缓存。
// 缓存读写失败时静默回退到 DB（err 由调用方降级为跳过检查）。
func loadApiKeyQuota(ctx context.Context, apiKeyID int64) (*apiKeyQuotaInfo, error) {
	key := apiKeyQuotaRedisKey(apiKeyID)

	result, err := g.Redis().Do(ctx, "HGETALL", key)
	if err == nil && !result.IsNil() && !result.IsEmpty() {
		m := result.MapStrVar()
		if tv, ok := m["total_quota"]; ok {
			return &apiKeyQuotaInfo{
				TotalQuota: tv.Float64(),
				UsedQuota:  m["used_quota"].Float64(),
			}, nil
		}
	}

	var row *struct {
		TotalQuota float64 `json:"total_quota"`
		UsedQuota  float64 `json:"used_quota"`
	}
	err = dao.ApiKeys.Ctx(ctx).
		Where("id", apiKeyID).
		Fields("COALESCE(total_quota, 0) AS total_quota, COALESCE(used_quota, 0) AS used_quota").
		Scan(&row)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return &apiKeyQuotaInfo{TotalQuota: 0, UsedQuota: 0}, nil
	}

	info := &apiKeyQuotaInfo{
		TotalQuota: row.TotalQuota,
		UsedQuota:  row.UsedQuota,
	}
	cacheApiKeyQuota(ctx, key, info)
	return info, nil
}

func cacheApiKeyQuota(ctx context.Context, key string, info *apiKeyQuotaInfo) {
	_, _ = g.Redis().Do(ctx, "HSET", key,
		"total_quota", info.TotalQuota,
		"used_quota", info.UsedQuota,
	)
	_, _ = g.Redis().Do(ctx, "EXPIRE", key, ApiKeyQuotaCacheTTL)
}

// InvalidateApiKeyQuotaCache 删除 API Key 额度的 Redis 缓存（额度累加落库后调用）。
func InvalidateApiKeyQuotaCache(ctx context.Context, apiKeyID int64) {
	key := apiKeyQuotaRedisKey(apiKeyID)
	_, _ = g.Redis().Do(ctx, "DEL", key)
}

// IncrApiKeyQuotaUsed increments an API key's used quota after settlement.
// 无条件累加（允许最后一笔超冲）：额度是控制线而非资金线，钱包预扣已兜底资金安全。
// 若按限额拒绝累加，used_quota 会永远停在限额之下，导致 CheckApiKeyQuota 永远放行、
// 额度被无限绕过；累加后超限由下一次 CheckApiKeyQuota 拦截。
// 累加动作放 fire-and-forget goroutine 异步执行：额度是控制线允许最终一致，不阻塞请求
// goroutine 与 DB 连接，落库方式与 RecordAudit 的异步审计写入保持一致。
// ctx 仅用于脱父级取消（WithoutCancel 保留链路值），客户端断开不再中断本次累加。
func IncrApiKeyQuotaUsed(ctx context.Context, apiKeyID int64, amount float64) {
	if apiKeyID <= 0 || amount <= 0 {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				g.Log().Errorf(context.Background(),
					"api_key_quota: incr panic apiKey=%d amount=%f: %v", apiKeyID, amount, r)
			}
		}()
		bgCtx := context.WithoutCancel(ctx)

		result, err := g.DB().Exec(bgCtx,
			`UPDATE api_keys
			 SET used_quota = COALESCE(used_quota, 0) + $1, updated_at = $2
			 WHERE id = $3`,
			amount, time.Now(), apiKeyID)
		if err != nil {
			g.Log().Errorf(bgCtx, "api_key_quota: atomic incr failed apiKey=%d amount=%f: %v", apiKeyID, amount, err)
			return
		}
		affected, err := result.RowsAffected()
		if err != nil {
			g.Log().Errorf(bgCtx, "api_key_quota: inspect incr result failed apiKey=%d: %v", apiKeyID, err)
			return
		}
		if affected == 0 {
			g.Log().Errorf(bgCtx, "api_key_quota: settlement increment target key not found apiKey=%d amount=%f", apiKeyID, amount)
			return
		}
		// 额度已累加，失效 Redis 缓存，使下一次 CheckApiKeyQuota 读到最新值
		InvalidateApiKeyQuotaCache(bgCtx, apiKeyID)
	}()
}
