package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyUpdate(ctx context.Context, req *v1.TenantApiKeyUpdateReq) (res *v1.TenantApiKeyUpdateRes, err error) {
	return service.Tenant().ApiKeyUpdate(ctx, req)
}
