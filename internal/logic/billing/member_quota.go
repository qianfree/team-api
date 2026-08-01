package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

const (
	MemberQuotaRedisKeyPrefix = "member_quota:"
	MemberQuotaCacheTTL       = 60 // seconds
)

type memberQuotaInfo struct {
	QuotaType    string  `json:"quota_type"`
	QuotaLimit   float64 `json:"quota_limit"`
	QuotaUsed    float64 `json:"quota_used"`
	QuotaPeriod  string  `json:"quota_period"`
	QuotaResetAt int64   `json:"quota_reset_at"` // unix timestamp, 0 = not set
}

func memberQuotaRedisKey(tenantID, userID int64) string {
	return fmt.Sprintf("%s%d:%d", MemberQuotaRedisKeyPrefix, tenantID, userID)
}

// CheckMemberQuota checks whether the member has sufficient quota.
// Returns nil if quota is sufficient or not configured, error if exceeded.
func CheckMemberQuota(ctx context.Context, tenantID, userID int64, preDeductAmount float64) error {
	info, err := loadMemberQuota(ctx, tenantID, userID)
	if err != nil {
		g.Log().Warningf(ctx, "member_quota: load failed tenant=%d user=%d: %v, skipping check", tenantID, userID, err)
		return nil
	}

	if info.QuotaType == "none" || info.QuotaType == "" {
		return nil
	}

	if info.QuotaType == "periodic" {
		if needsReset(info) {
			if resetMemberQuota(ctx, tenantID, userID) {
				info.QuotaUsed = 0
			}
			// 重置失败：保留旧 quota_used 参与本次判定（宁可误限一笔，不放飞额度），下次请求自动重试重置
		}
	}

	if info.QuotaUsed+preDeductAmount > info.QuotaLimit {
		return gerror.New("member quota exceeded")
	}

	return nil
}

// IncrMemberQuotaUsed increments the member's used quota after settlement.
// 无条件累加（允许最后一笔超冲）：额度是控制线而非资金线，钱包预扣已兜底资金安全。
// 若在此处按限额拒绝累加，quota_used 会永远停在限额之下，导致 CheckMemberQuota
// 永远放行、额度被无限绕过；累加后超限由下一次 CheckMemberQuota 拦截。
func IncrMemberQuotaUsed(ctx context.Context, tenantID, userID int64, amount float64) {
	if amount <= 0 {
		return
	}

	result, err := g.DB().Exec(ctx,
		`UPDATE tnt_users
		 SET quota_used = COALESCE(quota_used, 0) + $1, updated_at = $2
		 WHERE id = $3 AND tenant_id = $4`,
		amount, time.Now(), userID, tenantID)
	if err != nil {
		g.Log().Errorf(ctx, "member_quota: atomic incr failed tenant=%d user=%d amount=%f: %v", tenantID, userID, amount, err)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		g.Log().Errorf(ctx, "member_quota: inspect incr result failed tenant=%d user=%d: %v", tenantID, userID, err)
		return
	}
	if affected == 0 {
		g.Log().Errorf(ctx, "member_quota: settlement increment target user not found tenant=%d user=%d amount=%f", tenantID, userID, amount)
		return
	}
	InvalidateMemberQuotaCache(ctx, tenantID, userID)
}

// InvalidateMemberQuotaCache removes the Redis cache for a member's quota.
func InvalidateMemberQuotaCache(ctx context.Context, tenantID, userID int64) {
	key := memberQuotaRedisKey(tenantID, userID)
	_, _ = g.Redis().Do(ctx, "DEL", key)
}

func loadMemberQuota(ctx context.Context, tenantID, userID int64) (*memberQuotaInfo, error) {
	key := memberQuotaRedisKey(tenantID, userID)

	result, err := g.Redis().Do(ctx, "HGETALL", key)
	if err == nil && !result.IsNil() && !result.IsEmpty() {
		m := result.MapStrVar()
		if len(m) > 0 {
			if qt, ok := m["quota_type"]; ok && qt.String() != "" {
				return &memberQuotaInfo{
					QuotaType:    qt.String(),
					QuotaLimit:   m["quota_limit"].Float64(),
					QuotaUsed:    m["quota_used"].Float64(),
					QuotaPeriod:  m["quota_period"].String(),
					QuotaResetAt: m["quota_reset_at"].Int64(),
				}, nil
			}
		}
	}

	var row *struct {
		QuotaType    string     `json:"quota_type"`
		QuotaLimit   float64    `json:"quota_limit"`
		QuotaUsed    float64    `json:"quota_used"`
		QuotaPeriod  string     `json:"quota_period"`
		QuotaResetAt *time.Time `json:"quota_reset_at"`
	}
	err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Where("tenant_id", tenantID).
		Fields("quota_type, quota_limit, quota_used, quota_period, quota_reset_at").
		Scan(&row)
	if err != nil {
		return nil, err
	}

	if row == nil {
		return &memberQuotaInfo{QuotaType: "none"}, nil
	}

	info := &memberQuotaInfo{
		QuotaType:   row.QuotaType,
		QuotaLimit:  row.QuotaLimit,
		QuotaUsed:   row.QuotaUsed,
		QuotaPeriod: row.QuotaPeriod,
	}
	if row.QuotaResetAt != nil {
		info.QuotaResetAt = row.QuotaResetAt.Unix()
	}

	cacheMemberQuota(ctx, key, info)
	return info, nil
}

func cacheMemberQuota(ctx context.Context, key string, info *memberQuotaInfo) {
	pipe := g.Redis()
	_, _ = pipe.Do(ctx, "HSET", key,
		"quota_type", info.QuotaType,
		"quota_limit", info.QuotaLimit,
		"quota_used", info.QuotaUsed,
		"quota_period", info.QuotaPeriod,
		"quota_reset_at", info.QuotaResetAt,
	)
	_, _ = pipe.Do(ctx, "EXPIRE", key, MemberQuotaCacheTTL)
}

func needsReset(info *memberQuotaInfo) bool {
	if info.QuotaPeriod == "" {
		return false
	}

	now := time.Now().UTC()
	resetAt := time.Unix(info.QuotaResetAt, 0).UTC()

	switch info.QuotaPeriod {
	case "day":
		return !sameDay(resetAt, now)
	case "week":
		return !sameWeek(resetAt, now)
	case "month":
		return !sameMonth(resetAt, now)
	}
	return false
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
}

func sameWeek(a, b time.Time) bool {
	ay, aw := a.ISOWeek()
	by, bw := b.ISOWeek()
	return ay == by && aw == bw
}

func sameMonth(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month()
}

// resetMemberQuota 周期额度重置：先同步落 DB（权威状态），成功后失效 Redis 缓存。
// DB 失败返回 false，调用方按旧 quota_used 继续判定，下次请求自动重试；
// 不再异步写 DB——进程退出/写失败会导致 Redis 缓存过期后读回未重置的旧值，重置状态回滚。
func resetMemberQuota(ctx context.Context, tenantID, userID int64) bool {
	now := time.Now().UTC()

	_, err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Where("tenant_id", tenantID).
		Data(do.TntUsers{
			QuotaUsed:    0,
			QuotaResetAt: gtime.New(now),
		}).
		Update()
	if err != nil {
		g.Log().Errorf(ctx, "member_quota: reset db failed tenant=%d user=%d: %v", tenantID, userID, err)
		return false
	}

	// DB 已重置：直接失效缓存，下次 loadMemberQuota 从 DB 读到新状态（比 HSET 局部字段更不易漂移）
	InvalidateMemberQuotaCache(ctx, tenantID, userID)
	return true
}
