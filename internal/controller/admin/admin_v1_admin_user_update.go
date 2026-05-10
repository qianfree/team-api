package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserUpdate(ctx context.Context, req *v1.AdminUserUpdateReq) (res *v1.AdminUserUpdateRes, err error) {
	return service.Admin().UpdateUser(ctx, req)
}
