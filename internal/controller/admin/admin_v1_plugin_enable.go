package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginEnable(ctx context.Context, req *v1.PluginEnableReq) (res *v1.PluginEnableRes, err error) {
	return service.Admin().PluginEnable(ctx, req)
}
