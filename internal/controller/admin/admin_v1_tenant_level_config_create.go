package admin

import (
	"context"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLevelConfigCreate(ctx context.Context, req *v1.TenantLevelConfigCreateReq) (res *v1.TenantLevelConfigCreateRes, err error) {
	return service.Admin().CreateTenantLevelConfig(ctx, req)
}
