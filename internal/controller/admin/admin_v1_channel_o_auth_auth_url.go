package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelOAuthAuthURL(ctx context.Context, req *v1.ChannelOAuthAuthURLReq) (res *v1.ChannelOAuthAuthURLRes, err error) {
	return service.Admin().ChannelOAuthAuthURL(ctx, req)
}
