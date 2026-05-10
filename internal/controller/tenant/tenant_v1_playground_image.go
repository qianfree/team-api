package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PlaygroundImage(ctx context.Context, req *v1.PlaygroundImageReq) (res *v1.PlaygroundImageRes, err error) {
	return service.Tenant().PlaygroundImage(ctx, req)
}
