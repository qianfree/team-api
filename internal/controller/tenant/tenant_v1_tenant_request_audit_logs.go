package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRequestAuditLogs(ctx context.Context, req *v1.TenantRequestAuditLogsReq) (res *v1.TenantRequestAuditLogsRes, err error) {
	return service.Tenant().TenantRequestAuditLogs(ctx, req)
}
