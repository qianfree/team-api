package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRegister(ctx context.Context, req *v1.TenantRegisterReq) (res *v1.TenantRegisterRes, err error) {
	return service.Tenant().Register(ctx, req)
}
