package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginDetail(ctx context.Context, req *v1.TenantPluginDetailReq) (res *v1.TenantPluginDetailRes, err error) {
	return service.Tenant().TenantPluginDetail(ctx, req)
}
