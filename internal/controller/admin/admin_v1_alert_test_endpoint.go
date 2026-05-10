package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AlertTest(ctx context.Context, req *v1.AlertTestReq) (res *v1.AlertTestRes, err error) {
	return service.Monitor().AlertTest(ctx, req)
}
