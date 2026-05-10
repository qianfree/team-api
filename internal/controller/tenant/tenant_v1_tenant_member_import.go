package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberImport(ctx context.Context, req *v1.TenantMemberImportReq) (res *v1.TenantMemberImportRes, err error) {
	return service.Tenant().MemberImport(ctx, req)
}
