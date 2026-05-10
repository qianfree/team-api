package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderCreate(ctx context.Context, req *v1.TenantOrderCreateReq) (res *v1.TenantOrderCreateRes, err error) {
	return service.Tenant().OrderCreate(ctx, req)
}
