package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OrderExport(ctx context.Context, req *v1.OrderExportReq) (res *v1.OrderExportRes, err error) {
	return service.Admin().ExportOrders(ctx, req)
}
