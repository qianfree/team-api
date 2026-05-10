package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ErrorLogBatchResolve(ctx context.Context, req *v1.ErrorLogBatchResolveReq) (res *v1.ErrorLogBatchResolveRes, err error) {
	return service.Admin().ErrorLogBatchResolve(ctx, req)
}
