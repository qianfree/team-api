package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Admin2FASetup(ctx context.Context, req *v1.Admin2FASetupReq) (res *v1.Admin2FASetupRes, err error) {
	return service.Admin().Setup2FA(ctx, req)
}
