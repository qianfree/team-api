package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserUpdateStatus(ctx context.Context, req *v1.AdminUserUpdateStatusReq) (res *v1.AdminUserUpdateStatusRes, err error) {
	return service.Admin().UpdateUserStatus(ctx, req)
}
