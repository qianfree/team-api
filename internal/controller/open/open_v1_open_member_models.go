package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberModels(ctx context.Context, req *v1.OpenMemberModelsReq) (res *v1.OpenMemberModelsRes, err error) {
	return service.Open().OpenMemberModels(ctx, req)
}
