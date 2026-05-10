package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberResetPassword(ctx context.Context, req *v1.TenantMemberResetPasswordReq) (res *v1.TenantMemberResetPasswordRes, err error) {
	return service.Tenant().ResetMemberPassword(ctx, req)
}
