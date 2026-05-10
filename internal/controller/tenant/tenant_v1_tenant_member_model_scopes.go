package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberModelScopes(ctx context.Context, req *v1.TenantMemberModelScopesReq) (res *v1.TenantMemberModelScopesRes, err error) {
	return service.Tenant().MemberModelScopes(ctx, req)
}
