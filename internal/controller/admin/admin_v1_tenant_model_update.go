package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantModelUpdate(ctx context.Context, req *v1.TenantModelUpdateReq) (res *v1.TenantModelUpdateRes, err error) {
	return service.Admin().UpdateTenantModel(ctx, req)
}
