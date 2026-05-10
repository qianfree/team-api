package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelToggle(ctx context.Context, req *v1.PaymentChannelToggleReq) (res *v1.PaymentChannelToggleRes, err error) {
	return service.Admin().TogglePaymentChannel(ctx, req)
}
