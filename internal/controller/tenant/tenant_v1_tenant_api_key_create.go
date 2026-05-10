package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyCreate(ctx context.Context, req *v1.TenantApiKeyCreateReq) (res *v1.TenantApiKeyCreateRes, err error) {
	return service.Tenant().ApiKeyCreate(ctx, req)
}
