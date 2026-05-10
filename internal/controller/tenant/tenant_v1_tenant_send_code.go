package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantSendCode(ctx context.Context, req *v1.TenantSendCodeReq) (res *v1.TenantSendCodeRes, err error) {
	return service.Tenant().SendCode(ctx, req)
}
