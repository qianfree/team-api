package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) RedemptionList(ctx context.Context, req *v1.RedemptionListReq) (res *v1.RedemptionListRes, err error) {
	return service.Admin().ListRedemptions(ctx, req)
}
func (c *ControllerV1) RedemptionCreate(ctx context.Context, req *v1.RedemptionCreateReq) (res *v1.RedemptionCreateRes, err error) {
	return service.Admin().BatchCreateRedemptions(ctx, req)
}
func (c *ControllerV1) RedemptionDisable(ctx context.Context, req *v1.RedemptionDisableReq) (res *v1.RedemptionDisableRes, err error) {
	return service.Admin().DisableRedemption(ctx, req)
}
func (c *ControllerV1) RedemptionUsages(ctx context.Context, req *v1.RedemptionUsagesReq) (res *v1.RedemptionUsagesRes, err error) {
	return service.Admin().ListRedemptionUsages(ctx, req)
}
func (c *ControllerV1) RedemptionExport(ctx context.Context, req *v1.RedemptionExportReq) (res *v1.RedemptionExportRes, err error) {
	return service.Admin().ExportRedemptions(ctx, req)
}
