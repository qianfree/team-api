package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLogin(ctx context.Context, req *v1.TenantLoginReq) (res *v1.TenantLoginRes, err error) {
	return service.Tenant().Login(ctx, req)
}
