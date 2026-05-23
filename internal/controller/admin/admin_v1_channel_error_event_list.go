package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelErrorEventList(ctx context.Context, req *v1.ChannelErrorEventListReq) (res *v1.ChannelErrorEventListRes, err error) {
	return service.Admin().ChannelErrorEventList(ctx, req)
}
