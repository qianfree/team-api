package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FASetup(ctx context.Context, req *v1.Tenant2FASetupReq) (res *v1.Tenant2FASetupRes, err error) {
	return service.Tenant().Setup2FA(ctx, req)
}
func (c *ControllerV1) Tenant2FAEnable(ctx context.Context, req *v1.Tenant2FAEnableReq) (res *v1.Tenant2FAEnableRes, err error) {
	return service.Tenant().Enable2FA(ctx, req)
}
func (c *ControllerV1) Tenant2FADisable(ctx context.Context, req *v1.Tenant2FADisableReq) (res *v1.Tenant2FADisableRes, err error) {
	return service.Tenant().Disable2FA(ctx, req)
}
func (c *ControllerV1) Tenant2FARegenerateBackupCodes(ctx context.Context, req *v1.Tenant2FARegenerateBackupCodesReq) (res *v1.Tenant2FARegenerateBackupCodesRes, err error) {
	return service.Tenant().RegenerateBackupCodes(ctx, req)
}
func (c *ControllerV1) Tenant2FAVerify(ctx context.Context, req *v1.Tenant2FAVerifyReq) (res *v1.Tenant2FAVerifyRes, err error) {
	return service.Tenant().Verify2FA(ctx, req)
}
func (c *ControllerV1) Tenant2FAConfirm(ctx context.Context, req *v1.Tenant2FAConfirmReq) (res *v1.Tenant2FAConfirmRes, err error) {
	return service.Tenant().ConfirmHighRisk(ctx, req)
}
func (c *ControllerV1) TenantLoginHistory(ctx context.Context, req *v1.TenantLoginHistoryReq) (res *v1.TenantLoginHistoryRes, err error) {
	return service.Tenant().LoginHistory(ctx, req)
}
