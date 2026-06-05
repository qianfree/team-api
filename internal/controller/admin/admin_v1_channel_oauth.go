package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelOAuthAuthURL(ctx context.Context, req *v1.ChannelOAuthAuthURLReq) (res *v1.ChannelOAuthAuthURLRes, err error) {
	return service.Admin().ChannelOAuthAuthURL(ctx, req)
}
func (c *ControllerV1) ChannelOAuthExchange(ctx context.Context, req *v1.ChannelOAuthExchangeReq) (res *v1.ChannelOAuthExchangeRes, err error) {
	return service.Admin().ChannelOAuthExchange(ctx, req)
}
func (c *ControllerV1) ChannelOAuthRefresh(ctx context.Context, req *v1.ChannelOAuthRefreshReq) (res *v1.ChannelOAuthRefreshRes, err error) {
	return service.Admin().ChannelOAuthRefresh(ctx, req)
}
