package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlanToggleRecommend(ctx context.Context, req *v1.PlanToggleRecommendReq) (res *v1.PlanToggleRecommendRes, err error) {
	return service.Admin().ToggleRecommend(ctx, req)
}
