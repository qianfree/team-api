package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ErrorLogResolve(ctx context.Context, req *v1.ErrorLogResolveReq) (res *v1.ErrorLogResolveRes, err error) {
	return service.Admin().ErrorLogResolve(ctx, req)
}
