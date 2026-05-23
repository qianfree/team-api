package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelErrorTrend(ctx context.Context, req *v1.ChannelErrorTrendReq) (res *v1.ChannelErrorTrendRes, err error) {
	return service.Admin().ChannelErrorTrend(ctx, req)
}
