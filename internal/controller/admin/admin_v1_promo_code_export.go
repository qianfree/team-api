package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeExport(ctx context.Context, req *v1.PromoCodeExportReq) (res *v1.PromoCodeExportRes, err error) {
	return service.Admin().ExportPromoCodes(ctx, req)
}
