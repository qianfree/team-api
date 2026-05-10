package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberInvite(ctx context.Context, req *v1.TenantMemberInviteReq) (res *v1.TenantMemberInviteRes, err error) {
	return service.Tenant().InviteMember(ctx, req)
}
