package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertOptions(ctx context.Context, req *v1.AlertOptionsReq) (res *v1.AlertOptionsRes, err error) {
	return service.Monitor().AlertOptions(ctx, req)
}
