package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantIPWhitelistGet(ctx context.Context, req *v1.TenantIPWhitelistGetReq) (res *v1.TenantIPWhitelistGetRes, err error) {
	return service.Tenant().GetIPWhitelist(ctx, req)
}
