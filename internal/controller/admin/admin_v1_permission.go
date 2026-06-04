package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminPermissionList(ctx context.Context, req *v1.AdminPermissionListReq) (res *v1.AdminPermissionListRes, err error) {
	return service.Admin().GetUserPermissions(ctx, req)
}
func (c *ControllerV1) AdminPermissionUpdate(ctx context.Context, req *v1.AdminPermissionUpdateReq) (res *v1.AdminPermissionUpdateRes, err error) {
	return service.Admin().UpdateUserPermissions(ctx, req)
}
func (c *ControllerV1) AdminDataScopeUpdate(ctx context.Context, req *v1.AdminDataScopeUpdateReq) (res *v1.AdminDataScopeUpdateRes, err error) {
	return service.Admin().UpdateUserDataScopes(ctx, req)
}
func (c *ControllerV1) AdminAllPermissions(ctx context.Context, req *v1.AdminAllPermissionsReq) (res *v1.AdminAllPermissionsRes, err error) {
	return service.Admin().GetAllPermissions(ctx, req)
}
