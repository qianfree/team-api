package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberModelScopesSet(ctx context.Context, req *v1.TenantMemberModelScopesSetReq) (res *v1.TenantMemberModelScopesSetRes, err error) {
	return service.Tenant().MemberModelScopesSet(ctx, req)
}
