package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorLatency(ctx context.Context, req *v1.MonitorLatencyReq) (res *v1.MonitorLatencyRes, err error) {
	return service.Monitor().Latency(ctx, req)
}
