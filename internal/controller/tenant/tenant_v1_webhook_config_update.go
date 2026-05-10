package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) WebhookConfigUpdate(ctx context.Context, req *v1.WebhookConfigUpdateReq) (res *v1.WebhookConfigUpdateRes, err error) {
	return service.Tenant().WebhookConfigUpdate(ctx, req)
}
