package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) WebhookDeliveryLogs(ctx context.Context, req *v1.WebhookDeliveryLogsReq) (res *v1.WebhookDeliveryLogsRes, err error) {
	return service.Tenant().WebhookDeliveryLogs(ctx, req)
}
