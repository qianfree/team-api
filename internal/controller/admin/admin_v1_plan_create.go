package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanCreate(ctx context.Context, req *v1.PlanCreateReq) (res *v1.PlanCreateRes, err error) {
	return service.Admin().CreatePlan(ctx, req)
}
