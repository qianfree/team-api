package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorDBPool(ctx context.Context, req *v1.MonitorDBPoolReq) (res *v1.MonitorDBPoolRes, err error) {
	return service.Monitor().DBPool(ctx, req)
}
