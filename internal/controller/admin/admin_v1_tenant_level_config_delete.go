package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLevelConfigDelete(ctx context.Context, req *v1.TenantLevelConfigDeleteReq) (res *v1.TenantLevelConfigDeleteRes, err error) {
	return service.Admin().DeleteTenantLevelConfig(ctx, req)
}
