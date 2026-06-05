package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantLevelConfigList(ctx context.Context, req *v1.TenantLevelConfigListReq) (res *v1.TenantLevelConfigListRes, err error) {
	return service.Admin().ListTenantLevelConfigs(ctx, req)
}
func (c *ControllerV1) TenantLevelConfigCreate(ctx context.Context, req *v1.TenantLevelConfigCreateReq) (res *v1.TenantLevelConfigCreateRes, err error) {
	return service.Admin().CreateTenantLevelConfig(ctx, req)
}
func (c *ControllerV1) TenantLevelConfigUpdate(ctx context.Context, req *v1.TenantLevelConfigUpdateReq) (res *v1.TenantLevelConfigUpdateRes, err error) {
	return service.Admin().UpdateTenantLevelConfig(ctx, req)
}
func (c *ControllerV1) TenantLevelConfigDelete(ctx context.Context, req *v1.TenantLevelConfigDeleteReq) (res *v1.TenantLevelConfigDeleteRes, err error) {
	return service.Admin().DeleteTenantLevelConfig(ctx, req)
}
