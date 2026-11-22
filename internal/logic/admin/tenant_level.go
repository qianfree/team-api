package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
)

func (s *sAdmin) ListTenantLevelConfigs(ctx context.Context, _ *v1.TenantLevelConfigListReq) (*v1.TenantLevelConfigListRes, error) {
	var configs []*entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).
		OrderAsc("level").
		Scan(&configs)
	if err != nil {
		return nil, err
	}

	list := make([]*v1.TenantLevelConfigItem, 0, len(configs))
	for _, c := range configs {
		list = append(list, &v1.TenantLevelConfigItem{
			ID:                          c.Id,
			Level:                       c.Level,
			Name:                        c.Name,
			CumulativeRechargeThreshold: billing.InexactFloat64(c.CumulativeRechargeThreshold),
			MaxMembers:                  c.MaxMembers,
			MaxConcurrency:              c.MaxConcurrency,
			PriceMultiplier:             billing.InexactFloat64(c.PriceMultiplier),
			SortOrder:                   c.SortOrder,
			CreatedAt:                   c.CreatedAt,
			UpdatedAt:                   c.UpdatedAt,
		})
	}

	return &v1.TenantLevelConfigListRes{List: list}, nil
}

func (s *sAdmin) CreateTenantLevelConfig(ctx context.Context, req *v1.TenantLevelConfigCreateReq) (*v1.TenantLevelConfigCreateRes, error) {
	// 检查 level 是否已存在
	count, _ := dao.TntTenantLevelConfigs.Ctx(ctx).Where("level", req.Level).Count()
	if count > 0 {
		return nil, common.NewBadRequestError("等级号已存在")
	}
	// 折扣语义收口：等级乘数仅支持 (0,1]，>1 的加价计费引擎不会应用（GetLevelPriceMultiplier
	// 归一化为 1.0），此处直接拒绝，避免「配置展示 1.5 但不生效」的口径分叉
	if req.PriceMultiplier <= 0 || req.PriceMultiplier > 1 {
		return nil, common.NewBadRequestError("价格乘数需在 (0,1] 区间（折扣，如 0.9=九折）")
	}

	result, err := dao.TntTenantLevelConfigs.Ctx(ctx).Insert(do.TntTenantLevelConfigs{
		Level:                       req.Level,
		Name:                        req.Name,
		CumulativeRechargeThreshold: req.CumulativeRechargeThreshold,
		MaxMembers:                  req.MaxMembers,
		MaxConcurrency:              req.MaxConcurrency,
		PriceMultiplier:             req.PriceMultiplier,
		SortOrder:                   req.SortOrder,
	})
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()

	// 等级号可能被存量租户持有（删配置不清租户级别值），重建配置后折扣需即时生效
	billing.ClearLevelPriceCache(ctx, req.Level)

	return &v1.TenantLevelConfigCreateRes{ID: id}, nil
}

func (s *sAdmin) UpdateTenantLevelConfig(ctx context.Context, req *v1.TenantLevelConfigUpdateReq) (*v1.TenantLevelConfigUpdateRes, error) {
	data := do.TntTenantLevelConfigs{}
	hasUpdate := false
	if req.Name != nil {
		data.Name = *req.Name
		hasUpdate = true
	}
	if req.CumulativeRechargeThreshold != nil {
		data.CumulativeRechargeThreshold = *req.CumulativeRechargeThreshold
		hasUpdate = true
	}
	if req.MaxMembers != nil {
		data.MaxMembers = *req.MaxMembers
		hasUpdate = true
	}
	if req.MaxConcurrency != nil {
		data.MaxConcurrency = *req.MaxConcurrency
		hasUpdate = true
	}
	if req.PriceMultiplier != nil {
		data.PriceMultiplier = *req.PriceMultiplier
		hasUpdate = true
	}
	if req.SortOrder != nil {
		data.SortOrder = *req.SortOrder
		hasUpdate = true
	}

	if !hasUpdate {
		return &v1.TenantLevelConfigUpdateRes{}, nil
	}

	// 折扣语义收口（同 CreateTenantLevelConfig）
	if req.PriceMultiplier != nil && (*req.PriceMultiplier <= 0 || *req.PriceMultiplier > 1) {
		return nil, common.NewBadRequestError("价格乘数需在 (0,1] 区间（折扣，如 0.9=九折）")
	}

	// price_multiplier 变更前取等级号，更新后按等级失效缓存
	clearCache := req.PriceMultiplier != nil
	var level int
	if clearCache {
		var config *entity.TntTenantLevelConfigs
		_ = dao.TntTenantLevelConfigs.Ctx(ctx).Where("id", req.ID).Fields("level").Scan(&config)
		if config != nil {
			level = config.Level
		}
	}

	_, err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("id", req.ID).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}

	// 折扣乘数变更即时生效：清除该等级下全部租户的价格缓存（否则旧折扣残留 ≤600s）
	if clearCache && level > 0 {
		billing.ClearLevelPriceCache(ctx, level)
	}
	return &v1.TenantLevelConfigUpdateRes{}, nil
}

func (s *sAdmin) DeleteTenantLevelConfig(ctx context.Context, req *v1.TenantLevelConfigDeleteReq) (*v1.TenantLevelConfigDeleteRes, error) {
	var config *entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).Where("id", req.ID).Scan(&config)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return &v1.TenantLevelConfigDeleteRes{}, nil
	}
	if config.Level == 1 {
		return nil, common.NewBadRequestError("不允许删除默认等级 LV1")
	}

	_, err = dao.TntTenantLevelConfigs.Ctx(ctx).Where("id", req.ID).Delete()
	if err != nil {
		return nil, err
	}

	// 删配置后该等级折扣失效（GetLevelPriceMultiplier 查不到配置返回 1.0），
	// 存量租户的价格缓存仍烘焙旧折扣，需即时清除
	billing.ClearLevelPriceCache(ctx, config.Level)

	return &v1.TenantLevelConfigDeleteRes{}, nil
}
