package billing

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

// GetTenantEffectiveLimits 获取租户实际生效的成员数上限和并发上限。
// max_members 为 0/NULL 时从等级配置取值；等级配置也为 0 表示无限制。
// 返回值 0 表示无限制。调用方需用 memberCount < maxMembers || maxMembers == 0 判断。
func GetTenantEffectiveLimits(ctx context.Context, tenantID int64) (maxMembers int, maxConcurrency int, err error) {
	var tenant *entity.TntTenants
	err = dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil {
		return 0, 0, err
	}
	if tenant == nil {
		return 0, 0, nil
	}

	// 租户自定义值优先（> 0 表示已自定义，0/NULL 表示跟随等级配置）
	if tenant.MaxMembers > 0 {
		maxMembers = tenant.MaxMembers
	}
	if tenant.MaxConcurrency > 0 {
		maxConcurrency = tenant.MaxConcurrency
	}

	// 未自定义的字段从等级配置取
	if tenant.MaxMembers <= 0 || tenant.MaxConcurrency <= 0 {
		var config *entity.TntTenantLevelConfigs
		_ = dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", tenant.Level).Scan(&config)
		if tenant.MaxMembers <= 0 {
			if config != nil {
				maxMembers = config.MaxMembers // 0 表示无限制
			} else {
				maxMembers = 10
			}
		}
		if tenant.MaxConcurrency <= 0 {
			if config != nil {
				maxConcurrency = config.MaxConcurrency // 0 表示无限制
			} else {
				maxConcurrency = 0
			}
		}
	}

	return maxMembers, maxConcurrency, nil
}

// GetTenantEffectiveLimitsByEntity 通过已查询的租户实体获取实际生效限制（避免重复查询租户）
// 返回值 0 表示无限制。
func GetTenantEffectiveLimitsByEntity(ctx context.Context, tenant *entity.TntTenants) (maxMembers int, maxConcurrency int) {
	if tenant == nil {
		return 0, 0
	}

	// 租户自定义值优先（> 0 表示已自定义，0/NULL 表示跟随等级配置）
	if tenant.MaxMembers > 0 {
		maxMembers = tenant.MaxMembers
	}
	if tenant.MaxConcurrency > 0 {
		maxConcurrency = tenant.MaxConcurrency
	}

	// 未自定义的字段从等级配置取
	if tenant.MaxMembers <= 0 || tenant.MaxConcurrency <= 0 {
		var config *entity.TntTenantLevelConfigs
		_ = dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", tenant.Level).Scan(&config)
		if tenant.MaxMembers <= 0 {
			if config != nil {
				maxMembers = config.MaxMembers // 0 表示无限制
			} else {
				maxMembers = 10
			}
		}
		if tenant.MaxConcurrency <= 0 {
			if config != nil {
				maxConcurrency = config.MaxConcurrency // 0 表示无限制
			} else {
				maxConcurrency = 0
			}
		}
	}

	return maxMembers, maxConcurrency
}

// GetTenantLevelConfig 获取指定等级的配置
func GetTenantLevelConfig(ctx context.Context, level int) (*entity.TntTenantLevelConfigs, error) {
	var config *entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("level", level).
		Scan(&config)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("level config not found for level %d", level)
	}
	return config, nil
}

// GetLevelPriceMultiplier 获取租户级别的价格乘数
// 查询失败时静默返回 1.0，不阻断计费
func GetLevelPriceMultiplier(ctx context.Context, tenantID int64) float64 {
	var tenant *entity.TntTenants
	if err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant); err != nil || tenant == nil {
		g.Log().Warningf(ctx, "[Billing] failed to read tenant level for multiplier: tenantID=%d, err=%v", tenantID, err)
		return 1.0
	}

	if tenant.Level <= 1 {
		return 1.0
	}

	var config *entity.TntTenantLevelConfigs
	if err := dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", tenant.Level).Scan(&config); err != nil || config == nil {
		g.Log().Warningf(ctx, "[Billing] failed to read level config for multiplier: level=%d, err=%v", tenant.Level, err)
		return 1.0
	}

	if config.PriceMultiplier > 0 && config.PriceMultiplier != 1.0 {
		return config.PriceMultiplier
	}
	return 1.0
}

// CheckAndUpgradeLevel 检查并升级租户等级（充值后调用）
// 升级策略：仅升不降 — max_members/max_concurrency 仅在新值 > 当前值时才更新
func CheckAndUpgradeLevel(ctx context.Context, tenantID int64) error {
	// 1. 读取租户当前等级
	var tenant *entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil {
		return err
	}
	if tenant == nil {
		return nil
	}

	// 2. 读取累计充值
	var wallet *entity.BilWallets
	err = dao.BilWallets.Ctx(ctx).Where("tenant_id", tenantID).Scan(&wallet)
	if err != nil {
		return err
	}
	if wallet == nil {
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
	zero := 0
	updateData := do.TntTenants{Level: newLevel}
	hasUpdate := true

	// max_members：0 时跟随等级配置（不更新），> 0 时仅升不降
	if tenant.MaxMembers > 0 {
		if config.MaxMembers == 0 {
			// 新等级允许无限成员，直接更新
			updateData.MaxMembers = &zero
			hasUpdate = true
		} else if config.MaxMembers > tenant.MaxMembers {
			updateData.MaxMembers = &config.MaxMembers
			hasUpdate = true
		}
	}

	// max_concurrency：0 时跟随等级配置（不更新），> 0 时仅升不降
	if tenant.MaxConcurrency > 0 {
		if config.MaxConcurrency == 0 {
			updateData.MaxConcurrency = &zero
			hasUpdate = true
		} else if config.MaxConcurrency > tenant.MaxConcurrency {
			updateData.MaxConcurrency = &config.MaxConcurrency
			hasUpdate = true
		}
	}

	if hasUpdate {
		_, err = dao.TntTenants.Ctx(ctx).
			Where("id", tenantID).
			Data(updateData).
			Update()
		if err != nil {
			return err
		}
	}

	// 7. 清除并发限制缓存
	_, _ = g.Redis().Do(ctx, "DEL", fmt.Sprintf("tenant:conc_limit:%d", tenantID))

	// 8. 清除该租户的价格缓存（级别变化可能影响折扣）
	modelPriceCache.DeleteByPattern(ctx, fmt.Sprintf("%d:*", tenantID))

	return nil
}

// RecalculateLevel 根据累计充值金额重新计算等级
func RecalculateLevel(ctx context.Context, cumulativeRecharge float64) (int, error) {
	var config *entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("cumulative_recharge_threshold <= ?", cumulativeRecharge).
		OrderDesc("level").
		Limit(1).
		Scan(&config)
	if err != nil {
		return 1, err
	}
	if config == nil {
		return 1, nil
	}
	return config.Level, nil
}
