package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantProfileUpdate(ctx context.Context, req *v1.TenantProfileUpdateReq) (res *v1.TenantProfileUpdateRes, err error) {
	return service.Tenant().UpdateProfile(ctx, req)
}
