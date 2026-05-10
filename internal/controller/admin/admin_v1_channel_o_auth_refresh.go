package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelOAuthRefresh(ctx context.Context, req *v1.ChannelOAuthRefreshReq) (res *v1.ChannelOAuthRefreshRes, err error) {
	return service.Admin().ChannelOAuthRefresh(ctx, req)
}
