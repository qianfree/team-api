package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PersonalModelDist(ctx context.Context, req *v1.PersonalModelDistReq) (res *v1.PersonalModelDistRes, err error) {
	return service.Tenant().PersonalModelDistribution(ctx, req)
}
