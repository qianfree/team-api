package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceSettingsUpdate(ctx context.Context, req *v1.DataGovernanceSettingsUpdateReq) (res *v1.DataGovernanceSettingsUpdateRes, err error) {
	return service.Admin().DataGovernanceSettingsUpdate(ctx, req)
}
