package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketClose(ctx context.Context, req *v1.TenantTicketCloseReq) (res *v1.TenantTicketCloseRes, err error) {
	return service.Tenant().TicketClose(ctx, req)
}
