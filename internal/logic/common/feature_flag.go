package common

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
)

// IsFeatureEnabled 检查功能是否启用
// 优先级：租户覆盖 > 套餐 > 默认
func IsFeatureEnabled(ctx context.Context, tenantID int64, featureKey string) bool {
	// 1. 租户级覆盖
	if tenantID > 0 {
		var flag struct {
			Enabled bool `json:"enabled"`
		}
		err := dao.PlnFeatureFlags.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("feature_key", featureKey).
			Where("source", "tenant").
			Scan(&flag)
		if err == nil && flag.Enabled {
			return true
		}
	}

	// 2. 查租户当前套餐
	if tenantID > 0 {
		var planID int64
		err := dao.PlnTenantPlans.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("status", "active").
			Fields("plan_id").
			Limit(1).
			Scan(&planID)
		if err == nil && planID > 0 {
			var flag struct {
				Enabled bool `json:"enabled"`
			}
			err = dao.PlnFeatureFlags.Ctx(ctx).
				Where("plan_id", planID).
				Where("feature_key", featureKey).
				Scan(&flag)
			if err == nil && flag.Enabled {
				return true
			}
		}
	}

	// 3. 默认值
	var flag struct {
		DefaultEnabled bool `json:"default_enabled"`
	}
	err := dao.PlnFeatureFlags.Ctx(ctx).
		Where("feature_key", featureKey).
		Where("source", "manual").
		Limit(1).
		Scan(&flag)
	if err != nil {
		return false
	}
	return flag.DefaultEnabled
}
