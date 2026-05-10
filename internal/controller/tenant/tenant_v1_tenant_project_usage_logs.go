package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectUsageLogs(ctx context.Context, req *v1.TenantProjectUsageLogsReq) (res *v1.TenantProjectUsageLogsRes, err error) {
	return service.Tenant().ProjectUsageLogs(ctx, req)
}
