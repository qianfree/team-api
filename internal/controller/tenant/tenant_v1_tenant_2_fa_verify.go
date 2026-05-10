package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FAVerify(ctx context.Context, req *v1.Tenant2FAVerifyReq) (res *v1.Tenant2FAVerifyRes, err error) {
	return service.Tenant().Verify2FA(ctx, req)
}
