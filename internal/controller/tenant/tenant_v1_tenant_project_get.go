package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectGet(ctx context.Context, req *v1.TenantProjectGetReq) (res *v1.TenantProjectGetRes, err error) {
	return service.Tenant().ProjectGet(ctx, req)
}
