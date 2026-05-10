package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUserResetPassword(ctx context.Context, req *v1.AdminUserResetPasswordReq) (res *v1.AdminUserResetPasswordRes, err error) {
	return service.Admin().ResetUserPassword(ctx, req)
}
