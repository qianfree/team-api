package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PersonalDashboard(ctx context.Context, req *v1.PersonalDashboardReq) (res *v1.PersonalDashboardRes, err error) {
	return service.Tenant().PersonalDashboard(ctx, req)
}
func (c *ControllerV1) PersonalTokenTrends(ctx context.Context, req *v1.PersonalTokenTrendsReq) (res *v1.PersonalTokenTrendsRes, err error) {
	return service.Tenant().PersonalTokenTrends(ctx, req)
}
func (c *ControllerV1) PersonalModelDist(ctx context.Context, req *v1.PersonalModelDistReq) (res *v1.PersonalModelDistRes, err error) {
	return service.Tenant().PersonalModelDistribution(ctx, req)
}
func (c *ControllerV1) PersonalApiKeyUsage(ctx context.Context, req *v1.PersonalApiKeyUsageReq) (res *v1.PersonalApiKeyUsageRes, err error) {
	return service.Tenant().PersonalApiKeyUsage(ctx, req)
}
