package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) FeedbackCreate(ctx context.Context, req *v1.FeedbackCreateReq) (res *v1.FeedbackCreateRes, err error) {
	return service.Tenant().CreateFeedback(ctx, req)
}
