package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantWallet(ctx context.Context, req *v1.TenantWalletReq) (res *v1.TenantWalletRes, err error) {
	return service.Tenant().Wallet(ctx, req)
}
