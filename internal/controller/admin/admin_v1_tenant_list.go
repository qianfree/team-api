package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantList(ctx context.Context, req *v1.TenantListReq) (res *v1.TenantListRes, err error) {
	return service.Admin().ListTenants(ctx, req)
}
