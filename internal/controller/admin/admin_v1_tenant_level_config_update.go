package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLevelConfigUpdate(ctx context.Context, req *v1.TenantLevelConfigUpdateReq) (res *v1.TenantLevelConfigUpdateRes, err error) {
	return service.Admin().UpdateTenantLevelConfig(ctx, req)
}
