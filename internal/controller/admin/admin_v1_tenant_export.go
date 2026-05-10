package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantExport(ctx context.Context, req *v1.TenantExportReq) (res *v1.TenantExportRes, err error) {
	return service.Admin().ExportTenants(ctx, req)
}
