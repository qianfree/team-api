package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantGroupsList(ctx context.Context, req *v1.TenantGroupsListReq) (res *v1.TenantGroupsListRes, err error) {
	return service.Admin().ListTenantGroups(ctx, req)
}
