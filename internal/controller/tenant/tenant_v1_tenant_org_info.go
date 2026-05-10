package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrgInfo(ctx context.Context, req *v1.TenantOrgInfoReq) (res *v1.TenantOrgInfoRes, err error) {
	return service.Tenant().GetOrgInfo(ctx, req)
}
