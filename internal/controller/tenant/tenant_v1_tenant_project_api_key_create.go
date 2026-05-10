package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectApiKeyCreate(ctx context.Context, req *v1.TenantProjectApiKeyCreateReq) (res *v1.TenantProjectApiKeyCreateRes, err error) {
	return service.Tenant().ProjectApiKeyCreate(ctx, req)
}
