package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FeedbackListAll(ctx context.Context, req *v1.FeedbackListAllReq) (res *v1.FeedbackListAllRes, err error) {
	return service.Admin().ListAllFeedbacks(ctx, req)
}
func (c *ControllerV1) FeedbackReply(ctx context.Context, req *v1.FeedbackReplyReq) (res *v1.FeedbackReplyRes, err error) {
	return service.Admin().ReplyToFeedback(ctx, req)
}
func (c *ControllerV1) FeedbackUpdateStatus(ctx context.Context, req *v1.FeedbackUpdateStatusReq) (res *v1.FeedbackUpdateStatusRes, err error) {
	return service.Admin().UpdateFeedbackStatus(ctx, req)
}
func (c *ControllerV1) FeedbackStats(ctx context.Context, req *v1.FeedbackStatsReq) (res *v1.FeedbackStatsRes, err error) {
	return service.Admin().GetFeedbackStats(ctx, req)
}
