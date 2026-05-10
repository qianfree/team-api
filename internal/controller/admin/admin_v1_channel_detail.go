package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelDetail(ctx context.Context, req *v1.ChannelDetailReq) (res *v1.ChannelDetailRes, err error) {
	return service.Admin().GetChannelDetail(ctx, req)
}
