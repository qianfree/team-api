package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OrderRefund(ctx context.Context, req *v1.OrderRefundReq) (res *v1.OrderRefundRes, err error) {
	return service.Admin().RefundOrder(ctx, req)
}
