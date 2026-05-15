package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyModelScopes(ctx context.Context, req *v1.TenantApiKeyModelScopesReq) (res *v1.TenantApiKeyModelScopesRes, err error) {
	return service.Tenant().ApiKeyModelScopes(ctx, req)
}
