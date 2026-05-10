package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginUninstall(ctx context.Context, req *v1.PluginUninstallReq) (res *v1.PluginUninstallRes, err error) {
	return service.Admin().PluginUninstall(ctx, req)
}
