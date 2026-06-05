package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantTaskList(ctx context.Context, req *v1.TenantTaskListReq) (res *v1.TenantTaskListRes, err error) {
	return service.Tenant().TenantTaskList(ctx, req)
}
func (c *ControllerV1) TenantTaskDetail(ctx context.Context, req *v1.TenantTaskDetailReq) (res *v1.TenantTaskDetailRes, err error) {
	return service.Tenant().TenantTaskDetail(ctx, req)
}
