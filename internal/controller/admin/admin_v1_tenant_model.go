package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantModelList(ctx context.Context, req *v1.TenantModelListReq) (res *v1.TenantModelListRes, err error) {
	return service.Admin().ListTenantModels(ctx, req)
}
func (c *ControllerV1) TenantModelBatchAssign(ctx context.Context, req *v1.TenantModelBatchAssignReq) (res *v1.TenantModelBatchAssignRes, err error) {
	return service.Admin().BatchAssignModels(ctx, req)
}
func (c *ControllerV1) TenantModelUpdate(ctx context.Context, req *v1.TenantModelUpdateReq) (res *v1.TenantModelUpdateRes, err error) {
	return service.Admin().UpdateTenantModel(ctx, req)
}
func (c *ControllerV1) TenantModelDelete(ctx context.Context, req *v1.TenantModelDeleteReq) (res *v1.TenantModelDeleteRes, err error) {
	return service.Admin().DeleteTenantModel(ctx, req)
}
func (c *ControllerV1) TenantAvailableModelsPreview(ctx context.Context, req *v1.TenantAvailableModelsPreviewReq) (res *v1.TenantAvailableModelsPreviewRes, err error) {
	return service.Admin().ListTenantAvailableModels(ctx, req)
}
