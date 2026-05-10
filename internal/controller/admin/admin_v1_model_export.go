package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelExport(ctx context.Context, req *v1.ModelExportReq) (res *v1.ModelExportRes, err error) {
	return service.Admin().ExportModels(ctx, req)
}
