package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberUsage(ctx context.Context, req *v1.TenantMemberUsageReq) (res *v1.TenantMemberUsageRes, err error) {
	return service.Tenant().GetMemberUsage(ctx, req)
}
