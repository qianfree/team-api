package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TaskRetry(ctx context.Context, req *v1.TaskRetryReq) (res *v1.TaskRetryRes, err error) {
	return service.Admin().TaskRetry(ctx, req)
}
