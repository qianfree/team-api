package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) UsageLogCleanupList(ctx context.Context, req *v1.UsageLogCleanupListReq) (res *v1.UsageLogCleanupListRes, err error) {
	return service.Admin().UsageLogCleanupList(ctx, req)
}
