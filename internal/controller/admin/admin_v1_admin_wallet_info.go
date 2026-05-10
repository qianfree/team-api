package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminWalletInfo(ctx context.Context, req *v1.AdminWalletInfoReq) (res *v1.AdminWalletInfoRes, err error) {
	return service.Admin().GetWalletInfo(ctx, req)
}
