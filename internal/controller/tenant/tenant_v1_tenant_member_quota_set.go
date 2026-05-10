package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberQuotaSet(ctx context.Context, req *v1.TenantMemberQuotaSetReq) (res *v1.TenantMemberQuotaSetRes, err error) {
	return service.Tenant().MemberQuotaSet(ctx, req)
}
