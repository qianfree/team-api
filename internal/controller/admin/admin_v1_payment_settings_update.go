package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PaymentSettingsUpdate(ctx context.Context, req *v1.PaymentSettingsUpdateReq) (res *v1.PaymentSettingsUpdateRes, err error) {
	return service.Admin().UpdatePaymentSettings(ctx, req)
}
