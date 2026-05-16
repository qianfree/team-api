package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminTenantLoginHistory(ctx context.Context, req *v1.AdminTenantLoginHistoryReq) (res *v1.AdminTenantLoginHistoryRes, err error) {
	return service.Admin().TenantLoginHistory(ctx, req)
}
