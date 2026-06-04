package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PromoCodeList(ctx context.Context, req *v1.PromoCodeListReq) (res *v1.PromoCodeListRes, err error) {
	return service.Admin().ListPromoCodes(ctx, req)
}
func (c *ControllerV1) PromoCodeCreate(ctx context.Context, req *v1.PromoCodeCreateReq) (res *v1.PromoCodeCreateRes, err error) {
	return service.Admin().CreatePromoCode(ctx, req)
}
func (c *ControllerV1) PromoCodeUpdate(ctx context.Context, req *v1.PromoCodeUpdateReq) (res *v1.PromoCodeUpdateRes, err error) {
	return service.Admin().UpdatePromoCode(ctx, req)
}
func (c *ControllerV1) PromoCodeUsages(ctx context.Context, req *v1.PromoCodeUsagesReq) (res *v1.PromoCodeUsagesRes, err error) {
	return service.Admin().GetPromoCodeUsages(ctx, req)
}
func (c *ControllerV1) PromoCodeExport(ctx context.Context, req *v1.PromoCodeExportReq) (res *v1.PromoCodeExportRes, err error) {
	return service.Admin().ExportPromoCodes(ctx, req)
}
