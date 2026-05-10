package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) WebhookConfigCreate(ctx context.Context, req *v1.WebhookConfigCreateReq) (res *v1.WebhookConfigCreateRes, err error) {
	return service.Tenant().WebhookConfigCreate(ctx, req)
}
