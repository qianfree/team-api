package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelKeyCreate(ctx context.Context, req *v1.ChannelKeyCreateReq) (res *v1.ChannelKeyCreateRes, err error) {
	return service.Admin().AddChannelKey(ctx, req)
}
