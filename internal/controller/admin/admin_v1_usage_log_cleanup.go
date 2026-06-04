package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) UsageLogCleanupCreate(ctx context.Context, req *v1.UsageLogCleanupCreateReq) (res *v1.UsageLogCleanupCreateRes, err error) {
	return service.Admin().UsageLogCleanupCreate(ctx, req)
}
func (c *ControllerV1) UsageLogCleanupList(ctx context.Context, req *v1.UsageLogCleanupListReq) (res *v1.UsageLogCleanupListRes, err error) {
	return service.Admin().UsageLogCleanupList(ctx, req)
}
func (c *ControllerV1) UsageLogCleanupCancel(ctx context.Context, req *v1.UsageLogCleanupCancelReq) (res *v1.UsageLogCleanupCancelRes, err error) {
	return service.Admin().UsageLogCleanupCancel(ctx, req)
}
