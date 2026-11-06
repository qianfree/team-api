package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) JSPluginConfig(ctx context.Context, req *v1.JSPluginConfigReq) (res *v1.JSPluginConfigRes, err error) {
	return service.Admin().JSPluginConfig(ctx, req)
}

func (c *ControllerV1) JSPluginConfigUpdate(ctx context.Context, req *v1.JSPluginConfigUpdateReq) (res *v1.JSPluginConfigUpdateRes, err error) {
	return service.Admin().JSPluginConfigUpdate(ctx, req)
}
