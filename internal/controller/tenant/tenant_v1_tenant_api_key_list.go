package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantApiKeyList(ctx context.Context, req *v1.TenantApiKeyListReq) (res *v1.TenantApiKeyListRes, err error) {
	return service.Tenant().ApiKeyList(ctx, req)
}
