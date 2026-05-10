package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminWalletList(ctx context.Context, req *v1.AdminWalletListReq) (res *v1.AdminWalletListRes, err error) {
	return service.Admin().GetTenantWallets(ctx, req)
}
