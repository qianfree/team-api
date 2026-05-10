package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboardTopTenants(ctx context.Context, req *v1.AdminDashboardTopTenantsReq) (res *v1.AdminDashboardTopTenantsRes, err error) {
	return service.Admin().GetTopTenants(ctx, req)
}
