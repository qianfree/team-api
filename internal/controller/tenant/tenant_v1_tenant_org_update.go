package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrgUpdate(ctx context.Context, req *v1.TenantOrgUpdateReq) (res *v1.TenantOrgUpdateRes, err error) {
	return service.Tenant().UpdateOrgInfo(ctx, req)
}
