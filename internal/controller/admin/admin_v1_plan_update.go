package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanUpdate(ctx context.Context, req *v1.PlanUpdateReq) (res *v1.PlanUpdateRes, err error) {
	return service.Admin().UpdatePlan(ctx, req)
}
