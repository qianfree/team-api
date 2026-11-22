package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelUpstreamModels(ctx context.Context, req *v1.ChannelUpstreamModelsReq) (res *v1.ChannelUpstreamModelsRes, err error) {
	return service.Admin().FetchChannelUpstreamModels(ctx, req)
}
