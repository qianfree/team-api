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
			Id:                          c.Id,
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

	_, err := dao.TntTenantLevelConfigs.Ctx(ctx).
		Where("id", req.Id).
		Data(data).
		Update()
	if err != nil {
		return nil, err
	}
	return &v1.TenantLevelConfigUpdateRes{}, nil
}

func (s *sAdmin) DeleteTenantLevelConfig(ctx context.Context, req *v1.TenantLevelConfigDeleteReq) (*v1.TenantLevelConfigDeleteRes, error) {
	var config *entity.TntTenantLevelConfigs
	err := dao.TntTenantLevelConfigs.Ctx(ctx).Where("id", req.Id).Scan(&config)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return &v1.TenantLevelConfigDeleteRes{}, nil
	}
	if config.Level == 1 {
		return nil, common.NewBadRequestError("不允许删除默认等级 LV1")
	}

	_, err = dao.TntTenantLevelConfigs.Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}
	return &v1.TenantLevelConfigDeleteRes{}, nil
}
