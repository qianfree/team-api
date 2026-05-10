package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUsageLogExport(ctx context.Context, req *v1.AdminUsageLogExportReq) (res *v1.AdminUsageLogExportRes, err error) {
	return service.Admin().ExportUsageLogs(ctx, req)
}
