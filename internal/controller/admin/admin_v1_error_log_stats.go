package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ErrorLogStats(ctx context.Context, req *v1.ErrorLogStatsReq) (res *v1.ErrorLogStatsRes, err error) {
	return service.Admin().ErrorLogStats(ctx, req)
}
