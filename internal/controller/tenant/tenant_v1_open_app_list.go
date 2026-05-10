package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppList(ctx context.Context, req *v1.OpenAppListReq) (res *v1.OpenAppListRes, err error) {
	return service.Tenant().OpenAppList(ctx, req)
}
