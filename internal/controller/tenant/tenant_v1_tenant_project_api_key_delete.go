package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectApiKeyDelete(ctx context.Context, req *v1.TenantProjectApiKeyDeleteReq) (res *v1.TenantProjectApiKeyDeleteRes, err error) {
	return service.Tenant().ProjectApiKeyDelete(ctx, req)
}
