package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelUpdate(ctx context.Context, req *v1.PaymentChannelUpdateReq) (res *v1.PaymentChannelUpdateRes, err error) {
	return service.Admin().UpdatePaymentChannel(ctx, req)
}
