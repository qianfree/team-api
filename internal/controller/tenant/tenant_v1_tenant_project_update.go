package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectUpdate(ctx context.Context, req *v1.TenantProjectUpdateReq) (res *v1.TenantProjectUpdateRes, err error) {
	return service.Tenant().ProjectUpdate(ctx, req)
}
