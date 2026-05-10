package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginList(ctx context.Context, req *v1.PluginListReq) (res *v1.PluginListRes, err error) {
	return service.Admin().PluginList(ctx, req)
}
