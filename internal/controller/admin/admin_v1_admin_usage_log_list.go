package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminUsageLogList(ctx context.Context, req *v1.AdminUsageLogListReq) (res *v1.AdminUsageLogListRes, err error) {
	return service.Admin().GetAllUsageLogs(ctx, req)
}
