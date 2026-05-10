package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberJoin(ctx context.Context, req *v1.TenantMemberJoinReq) (res *v1.TenantMemberJoinRes, err error) {
	return service.Tenant().JoinByInvite(ctx, req)
}
