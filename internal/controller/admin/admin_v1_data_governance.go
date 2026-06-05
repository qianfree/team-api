package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) DataGovernanceSettingsGet(ctx context.Context, req *v1.DataGovernanceSettingsGetReq) (res *v1.DataGovernanceSettingsGetRes, err error) {
	return service.Admin().DataGovernanceSettingsGet(ctx, req)
}
func (c *ControllerV1) DataGovernanceSettingsUpdate(ctx context.Context, req *v1.DataGovernanceSettingsUpdateReq) (res *v1.DataGovernanceSettingsUpdateRes, err error) {
	return service.Admin().DataGovernanceSettingsUpdate(ctx, req)
}
func (c *ControllerV1) DataGovernanceExport(ctx context.Context, req *v1.DataGovernanceExportReq) (res *v1.DataGovernanceExportRes, err error) {
	return service.Admin().DataGovernanceExport(ctx, req)
}
func (c *ControllerV1) DataGovernanceDeletion(ctx context.Context, req *v1.DataGovernanceDeletionReq) (res *v1.DataGovernanceDeletionRes, err error) {
	return service.Admin().DataGovernanceDeletion(ctx, req)
}
func (c *ControllerV1) DataGovernanceCleanup(ctx context.Context, req *v1.DataGovernanceCleanupReq) (res *v1.DataGovernanceCleanupRes, err error) {
	return service.Admin().DataGovernanceCleanup(ctx, req)
}
