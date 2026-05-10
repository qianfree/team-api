package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminPermissionUpdate(ctx context.Context, req *v1.AdminPermissionUpdateReq) (res *v1.AdminPermissionUpdateRes, err error) {
	return service.Admin().UpdateUserPermissions(ctx, req)
}
