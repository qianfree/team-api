package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantBudgetAlerts(ctx context.Context, req *v1.TenantBudgetAlertsReq) (res *v1.TenantBudgetAlertsRes, err error) {
	return service.Tenant().BudgetAlerts(ctx, req)
}
