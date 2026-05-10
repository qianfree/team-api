package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminAllPermissions(ctx context.Context, req *v1.AdminAllPermissionsReq) (res *v1.AdminAllPermissionsRes, err error) {
	return service.Admin().GetAllPermissions(ctx, req)
}
