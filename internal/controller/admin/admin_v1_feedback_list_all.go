package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FeedbackListAll(ctx context.Context, req *v1.FeedbackListAllReq) (res *v1.FeedbackListAllRes, err error) {
	return service.Admin().ListAllFeedbacks(ctx, req)
}
