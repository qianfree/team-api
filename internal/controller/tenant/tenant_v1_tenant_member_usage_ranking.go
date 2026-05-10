package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantMemberUsageRanking(ctx context.Context, req *v1.TenantMemberUsageRankingReq) (res *v1.TenantMemberUsageRankingRes, err error) {
	return service.Tenant().GetMemberUsageRanking(ctx, req)
}
