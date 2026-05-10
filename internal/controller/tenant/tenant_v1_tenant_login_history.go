package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLoginHistory(ctx context.Context, req *v1.TenantLoginHistoryReq) (res *v1.TenantLoginHistoryRes, err error) {
	return service.Tenant().LoginHistory(ctx, req)
}
