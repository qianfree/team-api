package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenKeyDelete(ctx context.Context, req *v1.OpenKeyDeleteReq) (res *v1.OpenKeyDeleteRes, err error) {
	return service.Open().OpenKeyDelete(ctx, req)
}
