package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminRoleList(ctx context.Context, req *v1.AdminRoleListReq) (res *v1.AdminRoleListRes, err error) {
	return service.Admin().ListRoles(ctx, req)
}
func (c *ControllerV1) AdminRoleDetail(ctx context.Context, req *v1.AdminRoleDetailReq) (res *v1.AdminRoleDetailRes, err error) {
	return service.Admin().GetRoleDetail(ctx, req)
}
func (c *ControllerV1) AdminRoleCreate(ctx context.Context, req *v1.AdminRoleCreateReq) (res *v1.AdminRoleCreateRes, err error) {
	return service.Admin().CreateRole(ctx, req)
}
func (c *ControllerV1) AdminRoleUpdate(ctx context.Context, req *v1.AdminRoleUpdateReq) (res *v1.AdminRoleUpdateRes, err error) {
	return service.Admin().UpdateRole(ctx, req)
}
func (c *ControllerV1) AdminRoleDelete(ctx context.Context, req *v1.AdminRoleDeleteReq) (res *v1.AdminRoleDeleteRes, err error) {
	return service.Admin().DeleteRole(ctx, req)
}
func (c *ControllerV1) AdminRoleStatusUpdate(ctx context.Context, req *v1.AdminRoleStatusUpdateReq) (res *v1.AdminRoleStatusUpdateRes, err error) {
	return service.Admin().UpdateRoleStatus(ctx, req)
}
func (c *ControllerV1) AdminRoleReset(ctx context.Context, req *v1.AdminRoleResetReq) (res *v1.AdminRoleResetRes, err error) {
	return service.Admin().ResetRoleDefaults(ctx, req)
}
func (c *ControllerV1) AdminUserRoleList(ctx context.Context, req *v1.AdminUserRoleListReq) (res *v1.AdminUserRoleListRes, err error) {
	return service.Admin().GetUserRoles(ctx, req)
}
func (c *ControllerV1) AdminUserRoleAssign(ctx context.Context, req *v1.AdminUserRoleAssignReq) (res *v1.AdminUserRoleAssignRes, err error) {
	return service.Admin().AssignUserRoles(ctx, req)
}
