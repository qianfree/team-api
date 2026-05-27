package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) ModelOptions(ctx context.Context, req *v1.ModelOptionsReq) (res *v1.ModelOptionsRes, err error) {
	return service.Admin().ListModelOptions(ctx, req)
}
