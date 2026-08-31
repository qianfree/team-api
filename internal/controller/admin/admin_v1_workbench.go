package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminWorkbenchSummary(ctx context.Context, req *v1.AdminWorkbenchSummaryReq) (res *v1.AdminWorkbenchSummaryRes, err error) {
	return service.Admin().GetWorkbenchSummary(ctx, req)
}
func (c *ControllerV1) AdminWorkbenchBadge(ctx context.Context, req *v1.AdminWorkbenchBadgeReq) (res *v1.AdminWorkbenchBadgeRes, err error) {
	return service.Admin().GetWorkbenchBadges(ctx, req)
}
