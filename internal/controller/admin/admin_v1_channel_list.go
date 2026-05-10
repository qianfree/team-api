package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelList(ctx context.Context, req *v1.ChannelListReq) (res *v1.ChannelListRes, err error) {
	return service.Admin().ListChannels(ctx, req)
}
