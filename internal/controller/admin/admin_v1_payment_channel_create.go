package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelCreate(ctx context.Context, req *v1.PaymentChannelCreateReq) (res *v1.PaymentChannelCreateRes, err error) {
	return service.Admin().CreatePaymentChannel(ctx, req)
}
