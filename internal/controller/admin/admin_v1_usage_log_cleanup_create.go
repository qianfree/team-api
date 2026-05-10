package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) UsageLogCleanupCreate(ctx context.Context, req *v1.UsageLogCleanupCreateReq) (res *v1.UsageLogCleanupCreateRes, err error) {
	return service.Admin().UsageLogCleanupCreate(ctx, req)
}
