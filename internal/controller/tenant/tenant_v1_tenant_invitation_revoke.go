package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantInvitationRevoke(ctx context.Context, req *v1.TenantInvitationRevokeReq) (res *v1.TenantInvitationRevokeRes, err error) {
	return service.Tenant().RevokeInvitation(ctx, req)
}
