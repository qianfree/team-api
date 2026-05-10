package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectList(ctx context.Context, req *v1.TenantProjectListReq) (res *v1.TenantProjectListRes, err error) {
	return service.Tenant().ProjectList(ctx, req)
}
