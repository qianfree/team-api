package admin

import (
	"context"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/service"
)

func (c *ControllerV1) PluginList(ctx context.Context, req *v1.PluginListReq) (res *v1.PluginListRes, err error) {
	return service.Admin().PluginList(ctx, req)
}
func (c *ControllerV1) PluginDetail(ctx context.Context, req *v1.PluginDetailReq) (res *v1.PluginDetailRes, err error) {
	return service.Admin().PluginDetail(ctx, req)
}
func (c *ControllerV1) PluginInstall(ctx context.Context, req *v1.PluginInstallReq) (res *v1.PluginInstallRes, err error) {
	return service.Admin().PluginInstall(ctx, req)
}
func (c *ControllerV1) PluginEnable(ctx context.Context, req *v1.PluginEnableReq) (res *v1.PluginEnableRes, err error) {
	return service.Admin().PluginEnable(ctx, req)
}
func (c *ControllerV1) PluginDisable(ctx context.Context, req *v1.PluginDisableReq) (res *v1.PluginDisableRes, err error) {
	return service.Admin().PluginDisable(ctx, req)
}
func (c *ControllerV1) PluginUninstall(ctx context.Context, req *v1.PluginUninstallReq) (res *v1.PluginUninstallRes, err error) {
	return service.Admin().PluginUninstall(ctx, req)
}
func (c *ControllerV1) PluginUpgrade(ctx context.Context, req *v1.PluginUpgradeReq) (res *v1.PluginUpgradeRes, err error) {
	return service.Admin().PluginUpgrade(ctx, req)
}
func (c *ControllerV1) PluginConfigUpdate(ctx context.Context, req *v1.PluginConfigUpdateReq) (res *v1.PluginConfigUpdateRes, err error) {
	return service.Admin().PluginConfigUpdate(ctx, req)
}
func (c *ControllerV1) PluginConfigSchema(ctx context.Context, req *v1.PluginConfigSchemaReq) (res *v1.PluginConfigSchemaRes, err error) {
	return service.Admin().PluginConfigSchema(ctx, req)
}
