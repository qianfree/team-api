package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TicketAssign(ctx context.Context, req *v1.TicketAssignReq) (res *v1.TicketAssignRes, err error) {
	return service.Admin().AssignTicket(ctx, req)
}
