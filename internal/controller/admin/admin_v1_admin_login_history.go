package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminLoginHistory(ctx context.Context, req *v1.AdminLoginHistoryReq) (res *v1.AdminLoginHistoryRes, err error) {
	return service.Admin().LoginHistory(ctx, req)
}
