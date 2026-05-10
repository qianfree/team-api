package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelList(ctx context.Context, req *v1.ModelListReq) (res *v1.ModelListRes, err error) {
	return service.Admin().ListModels(ctx, req)
}
