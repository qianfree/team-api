package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) CronJobExecutions(ctx context.Context, req *v1.CronJobExecutionsReq) (res *v1.CronJobExecutionsRes, err error) {
	return service.Admin().CronJobExecutions(ctx, req)
}
