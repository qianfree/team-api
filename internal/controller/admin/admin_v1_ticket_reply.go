package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TicketReply(ctx context.Context, req *v1.TicketReplyReq) (res *v1.TicketReplyRes, err error) {
	return service.Admin().ReplyToTicketAdmin(ctx, req)
}
