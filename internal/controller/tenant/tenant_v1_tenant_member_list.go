package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberList(ctx context.Context, req *v1.TenantMemberListReq) (res *v1.TenantMemberListRes, err error) {
	return service.Tenant().ListMembers(ctx, req)
}
