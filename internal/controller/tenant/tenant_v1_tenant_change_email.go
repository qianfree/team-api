package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantChangeEmail(ctx context.Context, req *v1.TenantChangeEmailReq) (res *v1.TenantChangeEmailRes, err error) {
	return service.Tenant().ChangeEmail(ctx, req)
}
