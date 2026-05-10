package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberDelete(ctx context.Context, req *v1.OpenMemberDeleteReq) (res *v1.OpenMemberDeleteRes, err error) {
	return service.Open().OpenMemberDelete(ctx, req)
}
