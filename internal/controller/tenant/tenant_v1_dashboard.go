package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantDashboard(ctx context.Context, req *v1.TenantDashboardReq) (res *v1.TenantDashboardRes, err error) {
	return service.Tenant().Dashboard(ctx, req)
}
func (c *ControllerV1) TenantTokenTrends(ctx context.Context, req *v1.TenantTokenTrendsReq) (res *v1.TenantTokenTrendsRes, err error) {
	return service.Tenant().TokenTrends(ctx, req)
}
func (c *ControllerV1) TenantModelDistribution(ctx context.Context, req *v1.TenantModelDistributionReq) (res *v1.TenantModelDistributionRes, err error) {
	return service.Tenant().ModelDistribution(ctx, req)
}
func (c *ControllerV1) TenantBalancePrediction(ctx context.Context, req *v1.TenantBalancePredictionReq) (res *v1.TenantBalancePredictionRes, err error) {
	return service.Tenant().BalancePrediction(ctx, req)
}
func (c *ControllerV1) TenantBudgetAlerts(ctx context.Context, req *v1.TenantBudgetAlertsReq) (res *v1.TenantBudgetAlertsRes, err error) {
	return service.Tenant().BudgetAlerts(ctx, req)
}
func (c *ControllerV1) TenantMemberUsageRanking(ctx context.Context, req *v1.TenantMemberUsageRankingReq) (res *v1.TenantMemberUsageRankingRes, err error) {
	return service.Tenant().GetMemberUsageRanking(ctx, req)
}
