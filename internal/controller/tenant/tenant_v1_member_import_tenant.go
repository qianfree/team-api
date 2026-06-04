package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantImportRecords(ctx context.Context, req *v1.TenantImportRecordsReq) (res *v1.TenantImportRecordsRes, err error) {
	return service.Tenant().ImportRecords(ctx, req)
}
func (c *ControllerV1) TenantImportRecordGet(ctx context.Context, req *v1.TenantImportRecordGetReq) (res *v1.TenantImportRecordGetRes, err error) {
	return service.Tenant().ImportRecordGet(ctx, req)
}
func (c *ControllerV1) TenantMemberImport(ctx context.Context, req *v1.TenantMemberImportReq) (res *v1.TenantMemberImportRes, err error) {
	return service.Tenant().MemberImport(ctx, req)
}
