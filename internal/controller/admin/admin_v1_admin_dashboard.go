package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboard(ctx context.Context, req *v1.AdminDashboardReq) (res *v1.AdminDashboardRes, err error) {
	return service.Admin().GetDashboardStats(ctx, req)
}
