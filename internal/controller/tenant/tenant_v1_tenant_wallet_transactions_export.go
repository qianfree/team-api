package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantWalletTransactionsExport(ctx context.Context, req *v1.TenantWalletTransactionsExportReq) (res *v1.TenantWalletTransactionsExportRes, err error) {
	return service.Tenant().ExportWalletTransactions(ctx, req)
}
