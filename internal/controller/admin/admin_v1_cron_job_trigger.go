package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) CronJobTrigger(ctx context.Context, req *v1.CronJobTriggerReq) (res *v1.CronJobTriggerRes, err error) {
	return service.Admin().CronJobTrigger(ctx, req)
}
