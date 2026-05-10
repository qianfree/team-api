package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantAuditConfigGet(ctx context.Context, req *v1.TenantAuditConfigGetReq) (res *v1.TenantAuditConfigGetRes, err error) {
	return service.Tenant().AuditConfigGet(ctx, req)
}
