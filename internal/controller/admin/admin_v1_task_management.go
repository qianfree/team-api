package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TaskList(ctx context.Context, req *v1.TaskListReq) (res *v1.TaskListRes, err error) {
	return service.Admin().TaskList(ctx, req)
}
func (c *ControllerV1) TaskDetail(ctx context.Context, req *v1.TaskDetailReq) (res *v1.TaskDetailRes, err error) {
	return service.Admin().TaskDetail(ctx, req)
}
func (c *ControllerV1) TaskCancel(ctx context.Context, req *v1.TaskCancelReq) (res *v1.TaskCancelRes, err error) {
	return service.Admin().TaskCancel(ctx, req)
}
