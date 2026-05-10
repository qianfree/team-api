package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginList(ctx context.Context, req *v1.TenantPluginListReq) (res *v1.TenantPluginListRes, err error) {
	return service.Tenant().TenantPluginList(ctx, req)
}
