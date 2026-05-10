package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelAbilitiesGet(ctx context.Context, req *v1.ChannelAbilitiesGetReq) (res *v1.ChannelAbilitiesGetRes, err error) {
	return service.Admin().GetChannelAbilities(ctx, req)
}
