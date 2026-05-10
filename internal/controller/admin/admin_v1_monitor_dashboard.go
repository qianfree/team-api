package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorDashboard(ctx context.Context, req *v1.MonitorDashboardReq) (res *v1.MonitorDashboardRes, err error) {
	return service.Monitor().Dashboard(ctx, req)
}
