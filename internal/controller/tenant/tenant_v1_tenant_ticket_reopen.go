package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketReopen(ctx context.Context, req *v1.TenantTicketReopenReq) (res *v1.TenantTicketReopenRes, err error) {
	return service.Tenant().TicketReopen(ctx, req)
}
