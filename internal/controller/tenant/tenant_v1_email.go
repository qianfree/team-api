package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantSendCode(ctx context.Context, req *v1.TenantSendCodeReq) (res *v1.TenantSendCodeRes, err error) {
	return service.Tenant().SendCode(ctx, req)
}
func (c *ControllerV1) TenantResetPassword(ctx context.Context, req *v1.TenantResetPasswordReq) (res *v1.TenantResetPasswordRes, err error) {
	return service.Tenant().ResetPassword(ctx, req)
}
func (c *ControllerV1) TenantChangeEmail(ctx context.Context, req *v1.TenantChangeEmailReq) (res *v1.TenantChangeEmailRes, err error) {
	return service.Tenant().ChangeEmail(ctx, req)
}
