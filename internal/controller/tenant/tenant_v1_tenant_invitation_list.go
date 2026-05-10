package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantInvitationList(ctx context.Context, req *v1.TenantInvitationListReq) (res *v1.TenantInvitationListRes, err error) {
	return service.Tenant().InvitationList(ctx, req)
}
