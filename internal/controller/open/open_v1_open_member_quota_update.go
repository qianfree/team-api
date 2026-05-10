package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberQuotaUpdate(ctx context.Context, req *v1.OpenMemberQuotaUpdateReq) (res *v1.OpenMemberQuotaUpdateRes, err error) {
	return service.Open().OpenMemberQuotaUpdate(ctx, req)
}
