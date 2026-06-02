package tenant

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"

	v1 "github.com/qianfree/team-api/api/tenant/v1"

	"github.com/qianfree/team-api/internal/middleware"
)

// PlanList 获取可购买的套餐列表（仅 active）
func (s *sTenant) PlanList(ctx context.Context, req *v1.TenantPlanListReq) (*v1.TenantPlanListRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	var entities []*entity.PlnPlans
	err := dao.PlnPlans.Ctx(ctx).
		Where("status", "active").
		OrderAsc("sort_order").
		Scan(&entities)
	if err != nil {
		return nil, err
	}
	list := make([]*v1.TenantPlanItem, 0, len(entities))
	for _, e := range entities {
		list = append(list, &v1.TenantPlanItem{
			Id:                 e.Id,
			Name:               e.Name,
			Identifier:         e.Identifier,
			Description:        e.Description,
			MonthlyPrice:       e.MonthlyPrice,
			YearlyPrice:        e.YearlyPrice,
			MonthlyQuotaTokens: e.MonthlyQuotaTokens,
			AllowedModels:      e.AllowedModels,
			IsRecommended:      e.IsRecommended,
			SortOrder:          e.SortOrder,
		})
	}
	return &v1.TenantPlanListRes{List: list}, nil
}

// PlanCurrent 获取租户当前套餐
func (s *sTenant) PlanCurrent(ctx context.Context, req *v1.TenantPlanCurrentReq) (*v1.TenantPlanCurrentRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	var plan *v1.TenantPlanCurrentRes
	err := dao.PlnTenantPlans.Ctx(ctx).As("tp").
		Fields("tp.id, tp.tenant_id, tp.plan_id, tp.status, tp.start_at, tp.end_at, tp.auto_renew, tp.monthly_quota_tokens, tp.used_tokens, tp.last_reset_at, p.name, p.identifier, p.description, p.monthly_price, p.yearly_price").
		Where("tp.tenant_id", tenantID).
		Where("tp.status", "active").
		WhereIn("tp.plan_id", g.Model("pln_plans").Safe().Where("status", "active").Fields("id")).
		LeftJoin("pln_plans p", "p.id = tp.plan_id").
		OrderDesc("tp.start_at").
		Limit(1).
		Safe().
		Scan(&plan)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return nil, nil
	}
	return plan, nil
}

// PlanCancelAutoRenew 取消自动续费
func (s *sTenant) PlanCancelAutoRenew(ctx context.Context, req *v1.TenantPlanCancelAutoRenewReq) (*v1.TenantPlanCancelAutoRenewRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	tenantID := middleware.GetTenantID(ctx)
	_, err := dao.PlnTenantPlans.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Data(do.PlnTenantPlans{
			AutoRenew: false,
		}).Update()
	if err != nil {
		return nil, err
	}
	return &v1.TenantPlanCancelAutoRenewRes{}, nil
}
