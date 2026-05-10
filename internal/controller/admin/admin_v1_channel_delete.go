package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelDelete(ctx context.Context, req *v1.ChannelDeleteReq) (res *v1.ChannelDeleteRes, err error) {
	return service.Admin().DeleteChannel(ctx, req)
}
