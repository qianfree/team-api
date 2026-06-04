package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Admin2FASetup(ctx context.Context, req *v1.Admin2FASetupReq) (res *v1.Admin2FASetupRes, err error) {
	return service.Admin().Setup2FA(ctx, req)
}
func (c *ControllerV1) Admin2FAEnable(ctx context.Context, req *v1.Admin2FAEnableReq) (res *v1.Admin2FAEnableRes, err error) {
	return service.Admin().Enable2FA(ctx, req)
}
func (c *ControllerV1) Admin2FADisable(ctx context.Context, req *v1.Admin2FADisableReq) (res *v1.Admin2FADisableRes, err error) {
	return service.Admin().Disable2FA(ctx, req)
}
func (c *ControllerV1) Admin2FARegenerateBackupCodes(ctx context.Context, req *v1.Admin2FARegenerateBackupCodesReq) (res *v1.Admin2FARegenerateBackupCodesRes, err error) {
	return service.Admin().RegenerateBackupCodes(ctx, req)
}
func (c *ControllerV1) Admin2FAVerify(ctx context.Context, req *v1.Admin2FAVerifyReq) (res *v1.Admin2FAVerifyRes, err error) {
	return service.Admin().Verify2FA(ctx, req)
}
func (c *ControllerV1) Admin2FAConfirm(ctx context.Context, req *v1.Admin2FAConfirmReq) (res *v1.Admin2FAConfirmRes, err error) {
	return service.Admin().ConfirmHighRisk(ctx, req)
}
func (c *ControllerV1) AdminLoginHistory(ctx context.Context, req *v1.AdminLoginHistoryReq) (res *v1.AdminLoginHistoryRes, err error) {
	return service.Admin().LoginHistory(ctx, req)
}
func (c *ControllerV1) AdminTenantLoginHistory(ctx context.Context, req *v1.AdminTenantLoginHistoryReq) (res *v1.AdminTenantLoginHistoryRes, err error) {
	return service.Admin().TenantLoginHistory(ctx, req)
}
