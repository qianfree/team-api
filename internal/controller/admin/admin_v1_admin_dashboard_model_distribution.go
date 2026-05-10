package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboardModelDistribution(ctx context.Context, req *v1.AdminDashboardModelDistributionReq) (res *v1.AdminDashboardModelDistributionRes, err error) {
	return service.Admin().GetModelDistribution(ctx, req)
}
