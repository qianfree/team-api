package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantNotifications(ctx context.Context, req *v1.TenantNotificationsReq) (res *v1.TenantNotificationsRes, err error) {
	return service.Tenant().Notifications(ctx, req)
}
