package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) MessageSend(ctx context.Context, req *v1.MessageSendReq) (res *v1.MessageSendRes, err error) {
	return service.Admin().SendMessage(ctx, req)
}
