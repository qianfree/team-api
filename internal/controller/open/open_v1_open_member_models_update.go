package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberModelsUpdate(ctx context.Context, req *v1.OpenMemberModelsUpdateReq) (res *v1.OpenMemberModelsUpdateRes, err error) {
	return service.Open().OpenMemberModelsUpdate(ctx, req)
}
