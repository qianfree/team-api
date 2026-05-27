package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) GroupModelsList(ctx context.Context, req *v1.GroupModelsListReq) (res *v1.GroupModelsListRes, err error) {
	return service.Admin().ListGroupModels(ctx, req)
}
