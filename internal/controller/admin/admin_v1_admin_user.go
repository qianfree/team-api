package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserList(ctx context.Context, req *v1.AdminUserListReq) (res *v1.AdminUserListRes, err error) {
	return service.Admin().ListUsers(ctx, req)
}
func (c *ControllerV1) AdminUserCreate(ctx context.Context, req *v1.AdminUserCreateReq) (res *v1.AdminUserCreateRes, err error) {
	return service.Admin().CreateUser(ctx, req)
}
func (c *ControllerV1) AdminUserUpdate(ctx context.Context, req *v1.AdminUserUpdateReq) (res *v1.AdminUserUpdateRes, err error) {
	return service.Admin().UpdateUser(ctx, req)
}
func (c *ControllerV1) AdminUserDelete(ctx context.Context, req *v1.AdminUserDeleteReq) (res *v1.AdminUserDeleteRes, err error) {
	return service.Admin().DeleteUser(ctx, req)
}
func (c *ControllerV1) AdminUserUpdateStatus(ctx context.Context, req *v1.AdminUserUpdateStatusReq) (res *v1.AdminUserUpdateStatusRes, err error) {
	return service.Admin().UpdateUserStatus(ctx, req)
}
func (c *ControllerV1) AdminUserResetPassword(ctx context.Context, req *v1.AdminUserResetPasswordReq) (res *v1.AdminUserResetPasswordRes, err error) {
	return service.Admin().ResetUserPassword(ctx, req)
}
func (c *ControllerV1) AdminUserExport(ctx context.Context, req *v1.AdminUserExportReq) (res *v1.AdminUserExportRes, err error) {
	return service.Admin().ExportUsers(ctx, req)
}
