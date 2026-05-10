package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantResetPassword(ctx context.Context, req *v1.TenantResetPasswordReq) (res *v1.TenantResetPasswordRes, err error) {
	return service.Tenant().ResetPassword(ctx, req)
}
