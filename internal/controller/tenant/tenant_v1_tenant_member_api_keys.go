package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberApiKeys(ctx context.Context, req *v1.TenantMemberApiKeysReq) (res *v1.TenantMemberApiKeysRes, err error) {
	return service.Tenant().ListMemberApiKeys(ctx, req)
}
