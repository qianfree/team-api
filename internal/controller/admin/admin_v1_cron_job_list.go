package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) CronJobList(ctx context.Context, req *v1.CronJobListReq) (res *v1.CronJobListRes, err error) {
	return service.Admin().CronJobList(ctx, req)
}
