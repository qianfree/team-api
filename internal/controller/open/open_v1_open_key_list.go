package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenKeyList(ctx context.Context, req *v1.OpenKeyListReq) (res *v1.OpenKeyListRes, err error) {
	return service.Open().OpenKeyList(ctx, req)
}
