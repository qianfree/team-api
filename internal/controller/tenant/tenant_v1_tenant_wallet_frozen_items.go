package tenant

import (
	"context"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantWalletFrozenItems(ctx context.Context, req *v1.TenantWalletFrozenItemsReq) (res *v1.TenantWalletFrozenItemsRes, err error) {
	return service.Tenant().WalletFrozenItems(ctx, req)
}
