package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberList(ctx context.Context, req *v1.OpenMemberListReq) (res *v1.OpenMemberListRes, err error) {
	return service.Open().OpenMemberList(ctx, req)
}
