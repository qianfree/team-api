package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantModelBatchAssign(ctx context.Context, req *v1.TenantModelBatchAssignReq) (res *v1.TenantModelBatchAssignRes, err error) {
	return service.Admin().BatchAssignModels(ctx, req)
}
