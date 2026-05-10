package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelKeyDelete(ctx context.Context, req *v1.ChannelKeyDeleteReq) (res *v1.ChannelKeyDeleteRes, err error) {
	return service.Admin().DeleteChannelKey(ctx, req)
}
