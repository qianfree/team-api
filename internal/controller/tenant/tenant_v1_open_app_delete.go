package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppDelete(ctx context.Context, req *v1.OpenAppDeleteReq) (res *v1.OpenAppDeleteRes, err error) {
	return service.Tenant().OpenAppDelete(ctx, req)
}
