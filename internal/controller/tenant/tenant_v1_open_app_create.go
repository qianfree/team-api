package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenAppCreate(ctx context.Context, req *v1.OpenAppCreateReq) (res *v1.OpenAppCreateRes, err error) {
	return service.Tenant().OpenAppCreate(ctx, req)
}
