package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppToggleStatus(ctx context.Context, req *v1.OpenAppToggleStatusReq) (res *v1.OpenAppToggleStatusRes, err error) {
	return service.Tenant().OpenAppToggleStatus(ctx, req)
}
