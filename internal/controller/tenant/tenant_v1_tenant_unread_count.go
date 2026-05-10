package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantUnreadCount(ctx context.Context, req *v1.TenantUnreadCountReq) (res *v1.TenantUnreadCountRes, err error) {
	return service.Tenant().UnreadCount(ctx, req)
}
