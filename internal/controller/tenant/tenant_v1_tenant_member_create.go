package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberCreate(ctx context.Context, req *v1.TenantMemberCreateReq) (res *v1.TenantMemberCreateRes, err error) {
	return service.Tenant().CreateMember(ctx, req)
}
