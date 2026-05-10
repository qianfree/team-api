package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantBalancePrediction(ctx context.Context, req *v1.TenantBalancePredictionReq) (res *v1.TenantBalancePredictionRes, err error) {
	return service.Tenant().BalancePrediction(ctx, req)
}
