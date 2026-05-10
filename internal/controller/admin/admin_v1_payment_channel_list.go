package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelList(ctx context.Context, req *v1.PaymentChannelListReq) (res *v1.PaymentChannelListRes, err error) {
	return service.Admin().GetPaymentChannels(ctx, req)
}
