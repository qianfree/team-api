package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantGroupsSet(ctx context.Context, req *v1.TenantGroupsSetReq) (res *v1.TenantGroupsSetRes, err error) {
	return service.Admin().SetTenantGroups(ctx, req)
}
