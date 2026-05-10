package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceSettingsGet(ctx context.Context, req *v1.DataGovernanceSettingsGetReq) (res *v1.DataGovernanceSettingsGetRes, err error) {
	return service.Admin().DataGovernanceSettingsGet(ctx, req)
}
