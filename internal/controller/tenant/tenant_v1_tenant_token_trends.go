package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTokenTrends(ctx context.Context, req *v1.TenantTokenTrendsReq) (res *v1.TenantTokenTrendsRes, err error) {
	return service.Tenant().TokenTrends(ctx, req)
}
