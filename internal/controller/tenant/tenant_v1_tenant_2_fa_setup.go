package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FASetup(ctx context.Context, req *v1.Tenant2FASetupReq) (res *v1.Tenant2FASetupRes, err error) {
	return service.Tenant().Setup2FA(ctx, req)
}
