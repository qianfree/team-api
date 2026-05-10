package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppUpdate(ctx context.Context, req *v1.OpenAppUpdateReq) (res *v1.OpenAppUpdateRes, err error) {
	return service.Tenant().OpenAppUpdate(ctx, req)
}
