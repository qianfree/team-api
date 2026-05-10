package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRefresh(ctx context.Context, req *v1.TenantRefreshReq) (res *v1.TenantRefreshRes, err error) {
	return service.Tenant().Refresh(ctx, req)
}
