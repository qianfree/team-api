package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectApiKeyList(ctx context.Context, req *v1.TenantProjectApiKeyListReq) (res *v1.TenantProjectApiKeyListRes, err error) {
	return service.Tenant().ProjectApiKeyList(ctx, req)
}
