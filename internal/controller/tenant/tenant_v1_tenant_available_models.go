package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAvailableModels(ctx context.Context, req *v1.TenantAvailableModelsReq) (res *v1.TenantAvailableModelsRes, err error) {
	return service.Tenant().ListAvailableModels(ctx, req)
}
