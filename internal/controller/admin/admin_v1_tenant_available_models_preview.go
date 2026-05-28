package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAvailableModelsPreview(ctx context.Context, req *v1.TenantAvailableModelsPreviewReq) (res *v1.TenantAvailableModelsPreviewRes, err error) {
	return service.Admin().ListTenantAvailableModels(ctx, req)
}
