package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantRequestAuditLogDetail(ctx context.Context, req *v1.TenantRequestAuditLogDetailReq) (res *v1.TenantRequestAuditLogDetailRes, err error) {
	return service.Tenant().TenantRequestAuditLogDetail(ctx, req)
}
