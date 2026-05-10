package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TaskCancel(ctx context.Context, req *v1.TaskCancelReq) (res *v1.TaskCancelRes, err error) {
	return service.Admin().TaskCancel(ctx, req)
}
