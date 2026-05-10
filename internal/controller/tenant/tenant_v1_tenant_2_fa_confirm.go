package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) Tenant2FAConfirm(ctx context.Context, req *v1.Tenant2FAConfirmReq) (res *v1.Tenant2FAConfirmRes, err error) {
	return service.Tenant().ConfirmHighRisk(ctx, req)
}
