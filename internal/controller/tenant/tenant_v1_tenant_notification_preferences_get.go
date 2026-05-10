package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantNotificationPreferencesGet(ctx context.Context, req *v1.TenantNotificationPreferencesGetReq) (res *v1.TenantNotificationPreferencesGetRes, err error) {
	return service.Tenant().NotificationPreferencesGet(ctx, req)
}
