package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLevelBenefits(ctx context.Context, req *v1.TenantLevelBenefitsReq) (res *v1.TenantLevelBenefitsRes, err error) {
	return service.Tenant().GetLevelBenefits(ctx, req)
}
