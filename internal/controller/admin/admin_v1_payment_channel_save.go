package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentChannelSave(ctx context.Context, req *v1.PaymentChannelSaveReq) (res *v1.PaymentChannelSaveRes, err error) {
	return service.Admin().SavePaymentChannel(ctx, req)
}
