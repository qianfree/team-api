package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMarkRead(ctx context.Context, req *v1.TenantMarkReadReq) (res *v1.TenantMarkReadRes, err error) {
	return service.Tenant().MarkRead(ctx, req)
}
