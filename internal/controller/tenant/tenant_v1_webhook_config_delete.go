package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) WebhookConfigDelete(ctx context.Context, req *v1.WebhookConfigDeleteReq) (res *v1.WebhookConfigDeleteRes, err error) {
	return service.Tenant().WebhookConfigDelete(ctx, req)
}
