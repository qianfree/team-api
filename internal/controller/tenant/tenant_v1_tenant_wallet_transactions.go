package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantWalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsReq) (res *v1.TenantWalletTransactionsRes, err error) {
	return service.Tenant().WalletTransactions(ctx, req)
}
