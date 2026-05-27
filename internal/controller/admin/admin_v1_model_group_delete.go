package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelGroupDelete(ctx context.Context, req *v1.ModelGroupDeleteReq) (res *v1.ModelGroupDeleteRes, err error) {
	return service.Admin().DeleteModelGroup(ctx, req)
}
