package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceExport(ctx context.Context, req *v1.DataGovernanceExportReq) (res *v1.DataGovernanceExportRes, err error) {
	return service.Admin().DataGovernanceExport(ctx, req)
}
