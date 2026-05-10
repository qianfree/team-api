package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminLogout(ctx context.Context, req *v1.AdminLogoutReq) (res *v1.AdminLogoutRes, err error) {
	return service.Admin().Logout(ctx, req)
}
