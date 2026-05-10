package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectCreate(ctx context.Context, req *v1.TenantProjectCreateReq) (res *v1.TenantProjectCreateRes, err error) {
	return service.Tenant().ProjectCreate(ctx, req)
}
