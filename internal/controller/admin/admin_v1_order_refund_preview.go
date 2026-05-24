package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OrderRefundPreview(ctx context.Context, req *v1.OrderRefundPreviewReq) (res *v1.OrderRefundPreviewRes, err error) {
	return service.Admin().OrderRefundPreview(ctx, req)
}
