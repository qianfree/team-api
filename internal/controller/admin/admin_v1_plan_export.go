package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanExport(ctx context.Context, req *v1.PlanExportReq) (res *v1.PlanExportRes, err error) {
	return service.Admin().ExportPlans(ctx, req)
}
