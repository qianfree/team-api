package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAuditConfigGet(ctx context.Context, req *v1.TenantAuditConfigGetReq) (res *v1.TenantAuditConfigGetRes, err error) {
	return service.Tenant().AuditConfigGet(ctx, req)
}
func (c *ControllerV1) TenantAuditConfigUpdate(ctx context.Context, req *v1.TenantAuditConfigUpdateReq) (res *v1.TenantAuditConfigUpdateRes, err error) {
	return service.Tenant().AuditConfigUpdate(ctx, req)
}
func (c *ControllerV1) TenantAuditLogs(ctx context.Context, req *v1.TenantAuditLogsReq) (res *v1.TenantAuditLogsRes, err error) {
	return service.Tenant().AuditLogs(ctx, req)
}
func (c *ControllerV1) TenantRequestAuditLogs(ctx context.Context, req *v1.TenantRequestAuditLogsReq) (res *v1.TenantRequestAuditLogsRes, err error) {
	return service.Tenant().TenantRequestAuditLogs(ctx, req)
}
func (c *ControllerV1) TenantRequestAuditLogDetail(ctx context.Context, req *v1.TenantRequestAuditLogDetailReq) (res *v1.TenantRequestAuditLogDetailRes, err error) {
	return service.Tenant().TenantRequestAuditLogDetail(ctx, req)
}
