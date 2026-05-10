package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FeedbackUpdateStatus(ctx context.Context, req *v1.FeedbackUpdateStatusReq) (res *v1.FeedbackUpdateStatusRes, err error) {
	return service.Admin().UpdateFeedbackStatus(ctx, req)
}
