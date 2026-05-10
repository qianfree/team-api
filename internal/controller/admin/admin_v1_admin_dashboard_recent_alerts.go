package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminDashboardRecentAlerts(ctx context.Context, req *v1.AdminDashboardRecentAlertsReq) (res *v1.AdminDashboardRecentAlertsRes, err error) {
	return service.Admin().GetDashboardRecentAlerts(ctx, req)
}
