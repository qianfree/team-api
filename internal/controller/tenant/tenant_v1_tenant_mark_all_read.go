package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMarkAllRead(ctx context.Context, req *v1.TenantMarkAllReadReq) (res *v1.TenantMarkAllReadRes, err error) {
	return service.Tenant().MarkAllRead(ctx, req)
}
