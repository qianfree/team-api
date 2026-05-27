package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelGroupUpdate(ctx context.Context, req *v1.ModelGroupUpdateReq) (res *v1.ModelGroupUpdateRes, err error) {
	return service.Admin().UpdateModelGroup(ctx, req)
}
