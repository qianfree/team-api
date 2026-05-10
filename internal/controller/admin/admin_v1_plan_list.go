package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanList(ctx context.Context, req *v1.PlanListReq) (res *v1.PlanListRes, err error) {
	return service.Admin().ListPlans(ctx, req)
}
