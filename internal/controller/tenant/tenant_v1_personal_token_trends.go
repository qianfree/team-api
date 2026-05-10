package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PersonalTokenTrends(ctx context.Context, req *v1.PersonalTokenTrendsReq) (res *v1.PersonalTokenTrendsRes, err error) {
	return service.Tenant().PersonalTokenTrends(ctx, req)
}
