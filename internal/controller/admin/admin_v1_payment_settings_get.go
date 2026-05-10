package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentSettingsGet(ctx context.Context, req *v1.PaymentSettingsGetReq) (res *v1.PaymentSettingsGetRes, err error) {
	return service.Admin().GetPaymentSettings(ctx, req)
}
