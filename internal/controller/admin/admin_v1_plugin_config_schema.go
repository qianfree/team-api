package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginConfigSchema(ctx context.Context, req *v1.PluginConfigSchemaReq) (res *v1.PluginConfigSchemaRes, err error) {
	return service.Admin().PluginConfigSchema(ctx, req)
}
