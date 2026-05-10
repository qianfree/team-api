package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginDisable(ctx context.Context, req *v1.TenantPluginDisableReq) (res *v1.TenantPluginDisableRes, err error) {
	return service.Tenant().TenantPluginDisable(ctx, req)
}
