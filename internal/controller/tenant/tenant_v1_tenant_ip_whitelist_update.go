package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantIPWhitelistUpdate(ctx context.Context, req *v1.TenantIPWhitelistUpdateReq) (res *v1.TenantIPWhitelistUpdateRes, err error) {
	return service.Tenant().UpdateIPWhitelist(ctx, req)
}
