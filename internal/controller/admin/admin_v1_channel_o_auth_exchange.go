package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelOAuthExchange(ctx context.Context, req *v1.ChannelOAuthExchangeReq) (res *v1.ChannelOAuthExchangeRes, err error) {
	return service.Admin().ChannelOAuthExchange(ctx, req)
}
