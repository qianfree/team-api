package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberCreate(ctx context.Context, req *v1.OpenMemberCreateReq) (res *v1.OpenMemberCreateRes, err error) {
	return service.Open().OpenMemberCreate(ctx, req)
}
