package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantUsageLogsExport(ctx context.Context, req *v1.TenantUsageLogsExportReq) (res *v1.TenantUsageLogsExportRes, err error) {
	return service.Tenant().ExportUsageLogs(ctx, req)
}
