package billing

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"
)

// GetTenantLevelConfig 获取指定等级的配置
func GetTenantLevelConfig(ctx context.Context, level int) (*entity.TntTenantLevelConfigs, error) {
	var config entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("level", level).
		Scan(&config)
	if err != nil {
		return nil, err
	}
	if config.Id == 0 {
		return nil, fmt.Errorf("level config not found for level %d", level)
	}
	return &config, nil
}

// CheckAndUpgradeLevel 检查并升级租户等级（充值后调用）
// 升级策略：仅升不降 — max_members/max_concurrency 仅在新值 > 当前值时才更新
func CheckAndUpgradeLevel(ctx context.Context, tenantID int64) error {
	// 1. 读取租户当前等级
	var tenant entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil {
		return err
	}
	if tenant.Id == 0 {
		return nil
	}

	// 2. 读取累计充值
	var wallet entity.BilWallets
	err = dao.BilWallets.Ctx(ctx).Where("tenant_id", tenantID).Scan(&wallet)
	if err != nil {
		return err
	}
	if wallet.Id == 0 {
		return nil
	}

	// 3. 根据累计充值计算应得等级
	newLevel, err := RecalculateLevel(ctx, wallet.CumulativeRecharge)
	if err != nil {
		return err
	}

	// 4. 仅当等级提升时才更新
	if newLevel <= tenant.Level {
		return nil
	}

	// 5. 获取新等级配置
	config, err := GetTenantLevelConfig(ctx, newLevel)
	if err != nil {
		return err
	}

	// 6. 构建更新数据：仅升不降策略
	updateData := g.Map{
		"level":      newLevel,
		"updated_at": "NOW()",
	}

	// max_members：新值 > 当前值才更新（0=无限，视为最大）
	if config.MaxMembers == 0 {
		// 新等级允许无限成员，直接更新
		updateData["max_members"] = 0
	} else if config.MaxMembers > tenant.MaxMembers {
		updateData["max_members"] = config.MaxMembers
	}

	// max_concurrency：新值 > 当前值才更新（0=无限，视为最大）
	if config.MaxConcurrency == 0 {
		updateData["max_concurrency"] = 0
	} else if config.MaxConcurrency > tenant.MaxConcurrency {
		updateData["max_concurrency"] = config.MaxConcurrency
	}

	_, err = dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Data(updateData).
		Update()
	if err != nil {
		return err
	}

	// 7. 清除并发限制缓存
	_, _ = g.Redis().Do(ctx, "DEL", fmt.Sprintf("tenant:conc_limit:%d", tenantID))

	return nil
}

// RecalculateLevel 根据累计充值金额重新计算等级
func RecalculateLevel(ctx context.Context, cumulativeRecharge float64) (int, error) {
	var config entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("cumulative_recharge_threshold <= ?", cumulativeRecharge).
		OrderDesc("level").
		Limit(1).
		Scan(&config)
	if err != nil {
		return 1, err
	}
	if config.Id == 0 {
		return 1, nil
	}
	return config.Level, nil
}
