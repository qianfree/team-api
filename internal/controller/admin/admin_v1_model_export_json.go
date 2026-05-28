package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelExportJson(ctx context.Context, req *v1.ModelExportJsonReq) (res *v1.ModelExportJsonRes, err error) {
	return service.Admin().ExportModelsJson(ctx, req)
}
