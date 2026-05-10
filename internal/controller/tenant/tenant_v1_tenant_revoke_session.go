package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRevokeSession(ctx context.Context, req *v1.TenantRevokeSessionReq) (res *v1.TenantRevokeSessionRes, err error) {
	return service.Tenant().RevokeSession(ctx, req)
}
