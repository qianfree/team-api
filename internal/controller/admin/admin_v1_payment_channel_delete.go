package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelDelete(ctx context.Context, req *v1.PaymentChannelDeleteReq) (res *v1.PaymentChannelDeleteRes, err error) {
	return service.Admin().DeletePaymentChannel(ctx, req)
}
