package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenUsageQuery(ctx context.Context, req *v1.OpenUsageQueryReq) (res *v1.OpenUsageQueryRes, err error) {
	return service.Open().OpenUsageQuery(ctx, req)
}
