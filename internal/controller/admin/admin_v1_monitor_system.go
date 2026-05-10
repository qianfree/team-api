package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorSystem(ctx context.Context, req *v1.MonitorSystemReq) (res *v1.MonitorSystemRes, err error) {
	return service.Monitor().System(ctx, req)
}
