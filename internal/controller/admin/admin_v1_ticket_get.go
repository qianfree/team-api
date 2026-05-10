package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TicketGet(ctx context.Context, req *v1.TicketGetReq) (res *v1.TicketGetRes, err error) {
	return service.Admin().GetTicketAdmin(ctx, req)
}
