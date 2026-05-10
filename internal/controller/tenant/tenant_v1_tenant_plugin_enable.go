package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginEnable(ctx context.Context, req *v1.TenantPluginEnableReq) (res *v1.TenantPluginEnableRes, err error) {
	return service.Tenant().TenantPluginEnable(ctx, req)
}
