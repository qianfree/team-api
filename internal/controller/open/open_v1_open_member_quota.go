package open

import (
	"context"

	v1 "github.com/qianfree/team-api/api/open/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) OpenMemberQuota(ctx context.Context, req *v1.OpenMemberQuotaReq) (res *v1.OpenMemberQuotaRes, err error) {
	return service.Open().OpenMemberQuota(ctx, req)
}
