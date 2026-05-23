package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ChannelErrorCategories(ctx context.Context, req *v1.ChannelErrorCategoriesReq) (res *v1.ChannelErrorCategoriesRes, err error) {
	return service.Admin().ChannelErrorCategories(ctx, req)
}
