package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelList(ctx context.Context, req *v1.ModelListReq) (res *v1.ModelListRes, err error) {
	return service.Admin().ListModels(ctx, req)
}
func (c *ControllerV1) ModelCreate(ctx context.Context, req *v1.ModelCreateReq) (res *v1.ModelCreateRes, err error) {
	return service.Admin().CreateModel(ctx, req)
}
func (c *ControllerV1) ModelUpdate(ctx context.Context, req *v1.ModelUpdateReq) (res *v1.ModelUpdateRes, err error) {
	return service.Admin().UpdateModel(ctx, req)
}
func (c *ControllerV1) ModelDelete(ctx context.Context, req *v1.ModelDeleteReq) (res *v1.ModelDeleteRes, err error) {
	return service.Admin().DeleteModel(ctx, req)
}
func (c *ControllerV1) PricingGet(ctx context.Context, req *v1.PricingGetReq) (res *v1.PricingGetRes, err error) {
	return service.Admin().GetModelPricing(ctx, req)
}
func (c *ControllerV1) PricingSet(ctx context.Context, req *v1.PricingSetReq) (res *v1.PricingSetRes, err error) {
	return service.Admin().SetModelPricing(ctx, req)
}
func (c *ControllerV1) ModelOptions(ctx context.Context, req *v1.ModelOptionsReq) (res *v1.ModelOptionsRes, err error) {
	return service.Admin().ListModelOptions(ctx, req)
}
func (c *ControllerV1) ModelExport(ctx context.Context, req *v1.ModelExportReq) (res *v1.ModelExportRes, err error) {
	return service.Admin().ExportModels(ctx, req)
}
func (c *ControllerV1) PricingFetchOfficial(ctx context.Context, req *v1.PricingFetchOfficialReq) (res *v1.PricingFetchOfficialRes, err error) {
	return service.Admin().FetchOfficialPricing(ctx, req)
}
func (c *ControllerV1) ModelFetchOfficialInfo(ctx context.Context, req *v1.ModelFetchOfficialInfoReq) (res *v1.ModelFetchOfficialInfoRes, err error) {
	return service.Admin().FetchOfficialModelInfo(ctx, req)
}
func (c *ControllerV1) ModelExportJson(ctx context.Context, req *v1.ModelExportJsonReq) (res *v1.ModelExportJsonRes, err error) {
	return service.Admin().ExportModelsJson(ctx, req)
}
func (c *ControllerV1) ModelImportPreview(ctx context.Context, req *v1.ModelImportPreviewReq) (res *v1.ModelImportPreviewRes, err error) {
	return service.Admin().ImportModelsPreview(ctx, req)
}
func (c *ControllerV1) ModelImport(ctx context.Context, req *v1.ModelImportReq) (res *v1.ModelImportRes, err error) {
	return service.Admin().ImportModels(ctx, req)
}
