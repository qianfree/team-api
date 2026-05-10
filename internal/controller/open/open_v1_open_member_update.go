package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberUpdate(ctx context.Context, req *v1.OpenMemberUpdateReq) (res *v1.OpenMemberUpdateRes, err error) {
	return service.Open().OpenMemberUpdate(ctx, req)
}
