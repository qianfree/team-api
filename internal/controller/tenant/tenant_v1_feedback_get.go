package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FeedbackGet(ctx context.Context, req *v1.FeedbackGetReq) (res *v1.FeedbackGetRes, err error) {
	return service.Tenant().GetFeedback(ctx, req)
}
