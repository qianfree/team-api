package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorRealtime(ctx context.Context, req *v1.MonitorRealtimeReq) (res *v1.MonitorRealtimeRes, err error) {
	return service.Monitor().Realtime(ctx, req)
}
