package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OperationLogExport(ctx context.Context, req *v1.OperationLogExportReq) (res *v1.OperationLogExportRes, err error) {
	return service.Admin().ExportOperationLogs(ctx, req)
}
