package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelDetail(ctx context.Context, req *v1.PaymentChannelDetailReq) (res *v1.PaymentChannelDetailRes, err error) {
	return service.Admin().GetPaymentChannel(ctx, req)
}
