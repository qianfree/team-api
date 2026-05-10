package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketList(ctx context.Context, req *v1.TenantTicketListReq) (res *v1.TenantTicketListRes, err error) {
	return service.Tenant().TicketList(ctx, req)
}
