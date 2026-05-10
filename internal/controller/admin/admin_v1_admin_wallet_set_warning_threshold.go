package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) AdminWalletSetWarningThreshold(ctx context.Context, req *v1.AdminWalletSetWarningThresholdReq) (res *v1.AdminWalletSetWarningThresholdRes, err error) {
	return service.Admin().SetWarningThreshold(ctx, req)
}
