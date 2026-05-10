package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyDelete(ctx context.Context, req *v1.TenantApiKeyDeleteReq) (res *v1.TenantApiKeyDeleteRes, err error) {
	return service.Tenant().ApiKeyDelete(ctx, req)
}
