package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantNotificationPreferencesUpdate(ctx context.Context, req *v1.TenantNotificationPreferencesUpdateReq) (res *v1.TenantNotificationPreferencesUpdateRes, err error) {
	return service.Tenant().NotificationPreferencesUpdate(ctx, req)
}
