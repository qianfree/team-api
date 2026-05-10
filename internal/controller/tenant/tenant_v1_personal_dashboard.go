package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PersonalDashboard(ctx context.Context, req *v1.PersonalDashboardReq) (res *v1.PersonalDashboardRes, err error) {
	return service.Tenant().PersonalDashboard(ctx, req)
}
