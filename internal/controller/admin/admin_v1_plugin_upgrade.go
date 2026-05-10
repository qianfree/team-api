package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginUpgrade(ctx context.Context, req *v1.PluginUpgradeReq) (res *v1.PluginUpgradeRes, err error) {
	return service.Admin().PluginUpgrade(ctx, req)
}
