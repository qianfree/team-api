package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelCreate(ctx context.Context, req *v1.ChannelCreateReq) (res *v1.ChannelCreateRes, err error) {
	return service.Admin().CreateChannel(ctx, req)
}
