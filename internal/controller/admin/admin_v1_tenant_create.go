package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantCreate(ctx context.Context, req *v1.TenantCreateReq) (res *v1.TenantCreateRes, err error) {
	return service.Admin().CreateTenant(ctx, req)
}
