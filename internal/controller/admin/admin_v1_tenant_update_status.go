package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantUpdateStatus(ctx context.Context, req *v1.TenantUpdateStatusReq) (res *v1.TenantUpdateStatusRes, err error) {
	return service.Admin().UpdateTenantStatus(ctx, req)
}
