package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminForceLogout(ctx context.Context, req *v1.AdminForceLogoutReq) (res *v1.AdminForceLogoutRes, err error) {
	return service.Admin().ForceLogout(ctx, req)
}
