package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Admin2FADisable(ctx context.Context, req *v1.Admin2FADisableReq) (res *v1.Admin2FADisableRes, err error) {
	return service.Admin().Disable2FA(ctx, req)
}
