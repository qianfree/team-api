package tenant

import (
	"context"

	"github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) TenantPluginList(ctx context.Context, req *v1.TenantPluginListReq) (res *v1.TenantPluginListRes, err error) {
	return service.Tenant().TenantPluginList(ctx, req)
}
func (c *ControllerV1) TenantPluginDetail(ctx context.Context, req *v1.TenantPluginDetailReq) (res *v1.TenantPluginDetailRes, err error) {
	return service.Tenant().TenantPluginDetail(ctx, req)
}
func (c *ControllerV1) TenantPluginConfigUpdate(ctx context.Context, req *v1.TenantPluginConfigUpdateReq) (res *v1.TenantPluginConfigUpdateRes, err error) {
	return service.Tenant().TenantPluginConfigUpdate(ctx, req)
}
func (c *ControllerV1) TenantPluginEnable(ctx context.Context, req *v1.TenantPluginEnableReq) (res *v1.TenantPluginEnableRes, err error) {
	return service.Tenant().TenantPluginEnable(ctx, req)
}
func (c *ControllerV1) TenantPluginDisable(ctx context.Context, req *v1.TenantPluginDisableReq) (res *v1.TenantPluginDisableRes, err error) {
	return service.Tenant().TenantPluginDisable(ctx, req)
}
