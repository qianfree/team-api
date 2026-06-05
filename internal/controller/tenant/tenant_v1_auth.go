package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRegister(ctx context.Context, req *v1.TenantRegisterReq) (res *v1.TenantRegisterRes, err error) {
	return service.Tenant().Register(ctx, req)
}
func (c *ControllerV1) TenantLogin(ctx context.Context, req *v1.TenantLoginReq) (res *v1.TenantLoginRes, err error) {
	return service.Tenant().Login(ctx, req)
}
func (c *ControllerV1) TenantLogout(ctx context.Context, req *v1.TenantLogoutReq) (res *v1.TenantLogoutRes, err error) {
	return service.Tenant().Logout(ctx, req)
}
func (c *ControllerV1) TenantRefresh(ctx context.Context, req *v1.TenantRefreshReq) (res *v1.TenantRefreshRes, err error) {
	return service.Tenant().Refresh(ctx, req)
}
func (c *ControllerV1) TenantChangePassword(ctx context.Context, req *v1.TenantChangePasswordReq) (res *v1.TenantChangePasswordRes, err error) {
	return service.Tenant().ChangePassword(ctx, req)
}
func (c *ControllerV1) TenantSessionList(ctx context.Context, req *v1.TenantSessionListReq) (res *v1.TenantSessionListRes, err error) {
	return service.Tenant().ListSessions(ctx, req)
}
func (c *ControllerV1) TenantRevokeSession(ctx context.Context, req *v1.TenantRevokeSessionReq) (res *v1.TenantRevokeSessionRes, err error) {
	return service.Tenant().RevokeSession(ctx, req)
}
