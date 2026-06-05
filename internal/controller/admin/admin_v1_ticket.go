package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TicketList(ctx context.Context, req *v1.TicketListReq) (res *v1.TicketListRes, err error) {
	return service.Admin().ListAllTickets(ctx, req)
}
func (c *ControllerV1) TicketGet(ctx context.Context, req *v1.TicketGetReq) (res *v1.TicketGetRes, err error) {
	return service.Admin().GetTicketAdmin(ctx, req)
}
func (c *ControllerV1) TicketAssign(ctx context.Context, req *v1.TicketAssignReq) (res *v1.TicketAssignRes, err error) {
	return service.Admin().AssignTicket(ctx, req)
}
func (c *ControllerV1) TicketReply(ctx context.Context, req *v1.TicketReplyReq) (res *v1.TicketReplyRes, err error) {
	return service.Admin().ReplyToTicketAdmin(ctx, req)
}
func (c *ControllerV1) TicketStatusUpdate(ctx context.Context, req *v1.TicketStatusUpdateReq) (res *v1.TicketStatusUpdateRes, err error) {
	return service.Admin().UpdateTicketStatus(ctx, req)
}
