package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MonitorRedisPool(ctx context.Context, req *v1.MonitorRedisPoolReq) (res *v1.MonitorRedisPoolRes, err error) {
	return service.Monitor().RedisPool(ctx, req)
}
