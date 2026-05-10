package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantInviteInfo(ctx context.Context, req *v1.TenantInviteInfoReq) (res *v1.TenantInviteInfoRes, err error) {
	return service.Tenant().InviteInfo(ctx, req)
}
