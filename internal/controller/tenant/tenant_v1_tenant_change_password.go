package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantChangePassword(ctx context.Context, req *v1.TenantChangePasswordReq) (res *v1.TenantChangePasswordRes, err error) {
	return service.Tenant().ChangePassword(ctx, req)
}
