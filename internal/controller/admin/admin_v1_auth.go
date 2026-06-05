package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminLogin(ctx context.Context, req *v1.AdminLoginReq) (res *v1.AdminLoginRes, err error) {
	return service.Admin().Login(ctx, req)
}
func (c *ControllerV1) AdminLogout(ctx context.Context, req *v1.AdminLogoutReq) (res *v1.AdminLogoutRes, err error) {
	return service.Admin().Logout(ctx, req)
}
func (c *ControllerV1) AdminRefresh(ctx context.Context, req *v1.AdminRefreshReq) (res *v1.AdminRefreshRes, err error) {
	return service.Admin().Refresh(ctx, req)
}
func (c *ControllerV1) AdminSessionList(ctx context.Context, req *v1.AdminSessionListReq) (res *v1.AdminSessionListRes, err error) {
	return service.Admin().ListSessions(ctx, req)
}
func (c *ControllerV1) AdminRevokeSession(ctx context.Context, req *v1.AdminRevokeSessionReq) (res *v1.AdminRevokeSessionRes, err error) {
	return service.Admin().RevokeSession(ctx, req)
}
func (c *ControllerV1) AdminForceLogout(ctx context.Context, req *v1.AdminForceLogoutReq) (res *v1.AdminForceLogoutRes, err error) {
	return service.Admin().ForceLogout(ctx, req)
}
func (c *ControllerV1) AdminChangePassword(ctx context.Context, req *v1.AdminChangePasswordReq) (res *v1.AdminChangePasswordRes, err error) {
	return service.Admin().ChangePassword(ctx, req)
}
