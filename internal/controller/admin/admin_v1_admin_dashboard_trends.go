package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboardTrends(ctx context.Context, req *v1.AdminDashboardTrendsReq) (res *v1.AdminDashboardTrendsRes, err error) {
	return service.Admin().GetDashboardTrends(ctx, req)
}
