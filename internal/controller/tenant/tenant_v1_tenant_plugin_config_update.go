package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginConfigUpdate(ctx context.Context, req *v1.TenantPluginConfigUpdateReq) (res *v1.TenantPluginConfigUpdateRes, err error) {
	return service.Tenant().TenantPluginConfigUpdate(ctx, req)
}
