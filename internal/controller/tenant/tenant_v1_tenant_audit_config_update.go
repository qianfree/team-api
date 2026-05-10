package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAuditConfigUpdate(ctx context.Context, req *v1.TenantAuditConfigUpdateReq) (res *v1.TenantAuditConfigUpdateRes, err error) {
	return service.Tenant().AuditConfigUpdate(ctx, req)
}
