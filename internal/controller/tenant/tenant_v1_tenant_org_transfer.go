package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantOrgTransfer(ctx context.Context, req *v1.TenantOrgTransferReq) (res *v1.TenantOrgTransferRes, err error) {
	return service.Tenant().TransferOwnership(ctx, req)
}
