package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenKeyCreate(ctx context.Context, req *v1.OpenKeyCreateReq) (res *v1.OpenKeyCreateRes, err error) {
	return service.Open().OpenKeyCreate(ctx, req)
}
