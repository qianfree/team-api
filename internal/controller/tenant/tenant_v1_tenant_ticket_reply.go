package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTicketReply(ctx context.Context, req *v1.TenantTicketReplyReq) (res *v1.TenantTicketReplyRes, err error) {
	return service.Tenant().TicketReply(ctx, req)
}
