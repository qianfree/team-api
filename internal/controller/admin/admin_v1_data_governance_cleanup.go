package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceCleanup(ctx context.Context, req *v1.DataGovernanceCleanupReq) (res *v1.DataGovernanceCleanupRes, err error) {
	return service.Admin().DataGovernanceCleanup(ctx, req)
}
