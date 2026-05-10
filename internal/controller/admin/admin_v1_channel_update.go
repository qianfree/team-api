package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelUpdate(ctx context.Context, req *v1.ChannelUpdateReq) (res *v1.ChannelUpdateRes, err error) {
	return service.Admin().UpdateChannel(ctx, req)
}
