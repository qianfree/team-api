package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OrderComplete(ctx context.Context, req *v1.OrderCompleteReq) (res *v1.OrderCompleteRes, err error) {
	return service.Admin().OrderComplete(ctx, req)
}
