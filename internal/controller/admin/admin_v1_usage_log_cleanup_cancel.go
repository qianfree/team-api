package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) UsageLogCleanupCancel(ctx context.Context, req *v1.UsageLogCleanupCancelReq) (res *v1.UsageLogCleanupCancelRes, err error) {
	return service.Admin().UsageLogCleanupCancel(ctx, req)
}
