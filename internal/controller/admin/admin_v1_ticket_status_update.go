package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TicketStatusUpdate(ctx context.Context, req *v1.TicketStatusUpdateReq) (res *v1.TicketStatusUpdateRes, err error) {
	return service.Admin().UpdateTicketStatus(ctx, req)
}
