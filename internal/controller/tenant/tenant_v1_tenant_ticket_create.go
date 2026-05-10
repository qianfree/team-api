package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketCreate(ctx context.Context, req *v1.TenantTicketCreateReq) (res *v1.TenantTicketCreateRes, err error) {
	return service.Tenant().TicketCreate(ctx, req)
}
