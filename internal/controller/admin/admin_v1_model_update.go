package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelUpdate(ctx context.Context, req *v1.ModelUpdateReq) (res *v1.ModelUpdateRes, err error) {
	return service.Admin().UpdateModel(ctx, req)
}
