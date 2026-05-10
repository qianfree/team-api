package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrderExport(ctx context.Context, req *v1.TenantOrderExportReq) (res *v1.TenantOrderExportRes, err error) {
	return service.Tenant().ExportOrders(ctx, req)
}
