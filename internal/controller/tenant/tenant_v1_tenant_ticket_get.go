package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketGet(ctx context.Context, req *v1.TenantTicketGetReq) (res *v1.TenantTicketGetRes, err error) {
	return service.Tenant().TicketGet(ctx, req)
}
