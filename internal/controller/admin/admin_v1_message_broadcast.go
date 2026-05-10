package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MessageBroadcast(ctx context.Context, req *v1.MessageBroadcastReq) (res *v1.MessageBroadcastRes, err error) {
	return service.Admin().SendBroadcast(ctx, req)
}
