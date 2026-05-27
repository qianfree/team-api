package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) GroupModelsSet(ctx context.Context, req *v1.GroupModelsSetReq) (res *v1.GroupModelsSetRes, err error) {
	return service.Admin().SetGroupModels(ctx, req)
}
