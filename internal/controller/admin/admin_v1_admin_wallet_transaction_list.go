package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminWalletTransactionList(ctx context.Context, req *v1.AdminWalletTransactionListReq) (res *v1.AdminWalletTransactionListRes, err error) {
	return service.Admin().GetWalletTransactions(ctx, req)
}
