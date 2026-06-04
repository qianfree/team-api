package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProjectList(ctx context.Context, req *v1.TenantProjectListReq) (res *v1.TenantProjectListRes, err error) {
	return service.Tenant().ProjectList(ctx, req)
}
func (c *ControllerV1) TenantProjectCreate(ctx context.Context, req *v1.TenantProjectCreateReq) (res *v1.TenantProjectCreateRes, err error) {
	return service.Tenant().ProjectCreate(ctx, req)
}
func (c *ControllerV1) TenantProjectUpdate(ctx context.Context, req *v1.TenantProjectUpdateReq) (res *v1.TenantProjectUpdateRes, err error) {
	return service.Tenant().ProjectUpdate(ctx, req)
}
func (c *ControllerV1) TenantProjectArchive(ctx context.Context, req *v1.TenantProjectArchiveReq) (res *v1.TenantProjectArchiveRes, err error) {
	return service.Tenant().ProjectArchive(ctx, req)
}
func (c *ControllerV1) TenantProjectUnarchive(ctx context.Context, req *v1.TenantProjectUnarchiveReq) (res *v1.TenantProjectUnarchiveRes, err error) {
	return service.Tenant().ProjectUnarchive(ctx, req)
}
func (c *ControllerV1) TenantProjectGet(ctx context.Context, req *v1.TenantProjectGetReq) (res *v1.TenantProjectGetRes, err error) {
	return service.Tenant().ProjectGet(ctx, req)
}
func (c *ControllerV1) TenantProjectApiKeyList(ctx context.Context, req *v1.TenantProjectApiKeyListReq) (res *v1.TenantProjectApiKeyListRes, err error) {
	return service.Tenant().ProjectApiKeyList(ctx, req)
}
func (c *ControllerV1) TenantProjectApiKeyCreate(ctx context.Context, req *v1.TenantProjectApiKeyCreateReq) (res *v1.TenantProjectApiKeyCreateRes, err error) {
	return service.Tenant().ProjectApiKeyCreate(ctx, req)
}
func (c *ControllerV1) TenantProjectApiKeyDelete(ctx context.Context, req *v1.TenantProjectApiKeyDeleteReq) (res *v1.TenantProjectApiKeyDeleteRes, err error) {
	return service.Tenant().ProjectApiKeyDelete(ctx, req)
}
func (c *ControllerV1) TenantProjectUsageStats(ctx context.Context, req *v1.TenantProjectUsageStatsReq) (res *v1.TenantProjectUsageStatsRes, err error) {
	return service.Tenant().ProjectUsageStats(ctx, req)
}
func (c *ControllerV1) TenantProjectUsageLogs(ctx context.Context, req *v1.TenantProjectUsageLogsReq) (res *v1.TenantProjectUsageLogsRes, err error) {
	return service.Tenant().ProjectUsageLogs(ctx, req)
}
