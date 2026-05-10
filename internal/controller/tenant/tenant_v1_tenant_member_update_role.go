package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberUpdateRole(ctx context.Context, req *v1.TenantMemberUpdateRoleReq) (res *v1.TenantMemberUpdateRoleRes, err error) {
	return service.Tenant().UpdateMemberRole(ctx, req)
}
