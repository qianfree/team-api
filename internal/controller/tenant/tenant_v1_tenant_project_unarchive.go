package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectUnarchive(ctx context.Context, req *v1.TenantProjectUnarchiveReq) (res *v1.TenantProjectUnarchiveRes, err error) {
	return service.Tenant().ProjectUnarchive(ctx, req)
}
