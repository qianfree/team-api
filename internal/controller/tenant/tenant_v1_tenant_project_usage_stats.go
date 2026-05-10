package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectUsageStats(ctx context.Context, req *v1.TenantProjectUsageStatsReq) (res *v1.TenantProjectUsageStatsRes, err error) {
	return service.Tenant().ProjectUsageStats(ctx, req)
}
