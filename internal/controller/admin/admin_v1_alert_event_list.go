package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertEventList(ctx context.Context, req *v1.AlertEventListReq) (res *v1.AlertEventListRes, err error) {
	return service.Monitor().AlertEventList(ctx, req)
}
