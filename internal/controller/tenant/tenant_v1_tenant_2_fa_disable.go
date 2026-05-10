package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FADisable(ctx context.Context, req *v1.Tenant2FADisableReq) (res *v1.Tenant2FADisableRes, err error) {
	return service.Tenant().Disable2FA(ctx, req)
}
