package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) WebhookRetry(ctx context.Context, req *v1.WebhookRetryReq) (res *v1.WebhookRetryRes, err error) {
	return service.Tenant().WebhookRetry(ctx, req)
}
