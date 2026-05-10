package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelCreate(ctx context.Context, req *v1.ModelCreateReq) (res *v1.ModelCreateRes, err error) {
	return service.Admin().CreateModel(ctx, req)
}
