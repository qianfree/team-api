package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionExport(ctx context.Context, req *v1.RedemptionExportReq) (res *v1.RedemptionExportRes, err error) {
	return service.Admin().ExportRedemptions(ctx, req)
}
