package task

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/logic/tenant"
)

// LifecycleCron 生命周期定时扫描总入口
func LifecycleCron(ctx context.Context) {
	g.Log().Info(ctx, "[Lifecycle] starting cron scan...")
	CheckTrialExpiry(ctx)
	CheckGracePeriodExpiry(ctx)
	CheckFrozenExpiry(ctx)
	CheckClosingCooldown(ctx)
	g.Log().Info(ctx, "[Lifecycle] cron scan completed")
}

// CheckTrialExpiry 扫描试用到期
func CheckTrialExpiry(ctx context.Context) {
	now := time.Now()
	var count int
	_, err := dao.TntTenants.Ctx(ctx).
		Where("status", "trial").
		Where("trial_ends_at IS NOT NULL").
		Where("trial_ends_at < ?", now).
		Data(do.TntTenants{
			Status:        "frozen",
			FrozenAt:      gtime.NewFromTime(now),
			DataRemovalAt: gtime.NewFromTime(now.Add(30 * 24 * time.Hour)),
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[Lifecycle] CheckTrialExpiry: %v", err)
		return
	}
	// Note: RowsAffected not easily available, log generically
	if count > 0 || true {
		g.Log().Infof(ctx, "[Lifecycle] CheckTrialExpiry: processed trial expiries")
	}
}

// CheckGracePeriodExpiry 扫描宽限期到期
func CheckGracePeriodExpiry(ctx context.Context) {
	now := time.Now()
	_, err := dao.TntTenants.Ctx(ctx).
		Where("status", "past_due").
		Where("grace_period_ends_at IS NOT NULL").
		Where("grace_period_ends_at < ?", now).
		Data(do.TntTenants{
			Status:        "frozen",
			FrozenAt:      gtime.NewFromTime(now),
			DataRemovalAt: gtime.NewFromTime(now.Add(30 * 24 * time.Hour)),
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[Lifecycle] CheckGracePeriodExpiry: %v", err)
		return
	}
	g.Log().Info(ctx, "[Lifecycle] CheckGracePeriodExpiry: processed")
}

// CheckFrozenExpiry 扫描冻结到期（30 天后终止）
func CheckFrozenExpiry(ctx context.Context) {
	now := time.Now()
	_, err := dao.TntTenants.Ctx(ctx).
		Where("status", "frozen").
		Where("data_removal_at IS NOT NULL").
		Where("data_removal_at < ?", now).
		Data(do.TntTenants{
			Status: "terminated",
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[Lifecycle] CheckFrozenExpiry: %v", err)
		return
	}
	g.Log().Info(ctx, "[Lifecycle] CheckFrozenExpiry: processed")
}

// CheckClosingCooldown 扫描冷静期到期（7 天后关闭）
func CheckClosingCooldown(ctx context.Context) {
	now := time.Now()
	_, err := dao.TntTenants.Ctx(ctx).
		Where("status", "closing").
		Where("closing_requested_at IS NOT NULL").
		Where("closing_requested_at < ?", now.Add(-7*24*time.Hour)).
		Data(do.TntTenants{
			Status: "closed",
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "[Lifecycle] CheckClosingCooldown: %v", err)
		return
	}
	g.Log().Info(ctx, "[Lifecycle] CheckClosingCooldown: processed")
}

// EnsureTenantHasWallet 确保租户有钱包（用于解冻时检查）
func EnsureTenantActive(ctx context.Context, tenantID int64) {
	// Check if tenant has an active subscription
	var activePlan *struct {
		ID int64 `json:"id"`
	}
	err := dao.PlnTenantPlans.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Where("end_at > ?", time.Now()).
		Limit(1).
		Scan(&activePlan)
	if err == nil && activePlan != nil {
		// Has active plan, can unfreeze
		tenant.TransitionTenantStatus(ctx, tenantID, "active")
	}
}
