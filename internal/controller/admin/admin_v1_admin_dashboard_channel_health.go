package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboardChannelHealth(ctx context.Context, req *v1.AdminDashboardChannelHealthReq) (res *v1.AdminDashboardChannelHealthRes, err error) {
	return service.Admin().GetDashboardChannelHealth(ctx, req)
}
