package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelDelete(ctx context.Context, req *v1.ModelDeleteReq) (res *v1.ModelDeleteRes, err error) {
	return service.Admin().DeleteModel(ctx, req)
}
