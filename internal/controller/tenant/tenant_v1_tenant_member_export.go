package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberExport(ctx context.Context, req *v1.TenantMemberExportReq) (res *v1.TenantMemberExportRes, err error) {
	return service.Tenant().ExportMembers(ctx, req)
}
