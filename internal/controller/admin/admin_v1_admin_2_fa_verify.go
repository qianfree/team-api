package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Admin2FAVerify(ctx context.Context, req *v1.Admin2FAVerifyReq) (res *v1.Admin2FAVerifyRes, err error) {
	return service.Admin().Verify2FA(ctx, req)
}
