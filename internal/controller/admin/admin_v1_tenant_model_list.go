package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantModelList(ctx context.Context, req *v1.TenantModelListReq) (res *v1.TenantModelListRes, err error) {
	return service.Admin().ListTenantModels(ctx, req)
}
