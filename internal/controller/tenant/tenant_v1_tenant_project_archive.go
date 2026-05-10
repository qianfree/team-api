package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectArchive(ctx context.Context, req *v1.TenantProjectArchiveReq) (res *v1.TenantProjectArchiveRes, err error) {
	return service.Tenant().ProjectArchive(ctx, req)
}
