package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
)

// GetLevelBenefits returns all level configurations and the tenant's current level info.
func (s *sTenant) GetLevelBenefits(ctx context.Context, req *v1.TenantLevelBenefitsReq) (*v1.TenantLevelBenefitsRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	// Query all level configs ordered by level
	var configs []struct {
		Level                       int     `json:"level"`
		Name                        string  `json:"name"`
		CumulativeRechargeThreshold float64 `json:"cumulative_recharge_threshold"`
		MaxMembers                  int     `json:"max_members"`
		MaxConcurrency              int     `json:"max_concurrency"`
		PriceMultiplier             float64 `json:"price_multiplier"`
	}
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Order("level ASC").
		Fields("level, name, cumulative_recharge_threshold, max_members, max_concurrency, price_multiplier").
		Scan(&configs)
	if err != nil {
		return nil, err
	}

	// Query tenant's current level
	var tenant *struct {
		Level int `json:"level"`
	}
	err = dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Fields("level").
		Scan(&tenant)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, common.NewNotFoundError("租户")
	}

	// Query cumulative recharge from wallet
	var wallet *struct {
		CumulativeRecharge float64 `json:"cumulative_recharge"`
	}
	err = dao.BilWallets.Ctx(ctx).
		Where("tenant_id", tenantID).
		Fields("cumulative_recharge").
		Scan(&wallet)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	cumulativeRecharge := 0.0
	if wallet != nil {
		cumulativeRecharge = wallet.CumulativeRecharge
	}

	// Look up current level name
	var currentLevelName string
	for _, c := range configs {
		if c.Level == tenant.Level {
			currentLevelName = c.Name
			break
		}
	}

	items := make([]v1.TenantLevelBenefitItem, 0, len(configs))
	for _, c := range configs {
		items = append(items, v1.TenantLevelBenefitItem{
			Level:                       c.Level,
			Name:                        c.Name,
			CumulativeRechargeThreshold: c.CumulativeRechargeThreshold,
			MaxMembers:                  c.MaxMembers,
			MaxConcurrency:              c.MaxConcurrency,
			PriceMultiplier:             c.PriceMultiplier,
		})
	}

	return &v1.TenantLevelBenefitsRes{
		List:               items,
		CurrentLevel:       tenant.Level,
		CurrentLevelName:   currentLevelName,
		CumulativeRecharge: cumulativeRecharge,
	}, nil
}
