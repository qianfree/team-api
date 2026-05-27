package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelGroupOptions(ctx context.Context, req *v1.ModelGroupOptionsReq) (res *v1.ModelGroupOptionsRes, err error) {
	return service.Admin().ListModelGroupOptions(ctx, req)
}
