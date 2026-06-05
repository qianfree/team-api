package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantWallet(ctx context.Context, req *v1.TenantWalletReq) (res *v1.TenantWalletRes, err error) {
	return service.Tenant().Wallet(ctx, req)
}
func (c *ControllerV1) TenantWalletTransactions(ctx context.Context, req *v1.TenantWalletTransactionsReq) (res *v1.TenantWalletTransactionsRes, err error) {
	return service.Tenant().WalletTransactions(ctx, req)
}
func (c *ControllerV1) TenantWalletTransactionsExport(ctx context.Context, req *v1.TenantWalletTransactionsExportReq) (res *v1.TenantWalletTransactionsExportRes, err error) {
	return service.Tenant().ExportWalletTransactions(ctx, req)
}
func (c *ControllerV1) TenantWalletFrozenItems(ctx context.Context, req *v1.TenantWalletFrozenItemsReq) (res *v1.TenantWalletFrozenItemsRes, err error) {
	return service.Tenant().WalletFrozenItems(ctx, req)
}
