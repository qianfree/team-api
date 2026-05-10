package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantGet(ctx context.Context, req *v1.TenantGetReq) (res *v1.TenantGetRes, err error) {
	return service.Admin().GetTenant(ctx, req)
}
