package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantNotificationDelete(ctx context.Context, req *v1.TenantNotificationDeleteReq) (res *v1.TenantNotificationDeleteRes, err error) {
	return service.Tenant().DeleteNotification(ctx, req)
}
