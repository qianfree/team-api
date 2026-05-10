package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberRemove(ctx context.Context, req *v1.TenantMemberRemoveReq) (res *v1.TenantMemberRemoveRes, err error) {
	return service.Tenant().RemoveMember(ctx, req)
}
