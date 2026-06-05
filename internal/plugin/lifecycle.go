package plugin

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
)

// Install 安装插件（管理后台 API 调用触发）。
func Install(ctx context.Context, name string) error {
	entry := GetPlugin(name)
	if entry == nil {
		return gerror.New("插件不存在")
	}
	if entry.Status == StatusInstalled || entry.Status == StatusEnabled || entry.Status == StatusDisabled {
		return gerror.New("插件已安装")
	}

	if err := checkDependencies(entry); err != nil {
		return err
	}

	if err := entry.Plugin.Install(ctx); err != nil {
		return gerror.Wrap(err, "插件安装失败")
	}

	if c, ok := entry.Plugin.(Configurable); ok {
		if err := registerConfigSchema(ctx, name, c.ConfigSchema()); err != nil {
			return gerror.Wrap(err, "插件配置注册失败")
		}
	}

	_, err := dao.SysPlugins.Ctx(ctx).Data(do.SysPlugins{
		Name:     name,
		Label:    entry.Info.Label,
		Version:  entry.Info.Version,
		Status:   StatusInstalled,
		Category: entry.Info.Category,
		Config:   "{}",
	}).Insert()
	if err != nil {
		return gerror.Wrap(err, "写入安装记录失败")
	}

	entry.Status = StatusInstalled
	glog.Infof(ctx, "plugin %s: installed successfully", name)
	return nil
}

// Enable 启用插件。
func Enable(ctx context.Context, name string) error {
	entry := GetPlugin(name)
	if entry == nil {
		return gerror.New("插件不存在")
	}
	if entry.Status != StatusInstalled && entry.Status != StatusDisabled {
		return gerror.New("插件未安装，无法启用")
	}

	if err := startPlugin(ctx, entry); err != nil {
		_, _ = dao.SysPlugins.Ctx(ctx).
			Where("name", name).
			Data(do.SysPlugins{Status: StatusError, ErrorMsg: err.Error()}).
			Update()
		return err
	}

	_, err := dao.SysPlugins.Ctx(ctx).
		Where("name", name).
		Data(do.SysPlugins{Status: StatusEnabled, ErrorMsg: ""}).
		Update()
	return err
}

// Disable 禁用插件（不卸载，保留数据）。
func Disable(ctx context.Context, name string) error {
	entry := GetPlugin(name)
	if entry == nil {
		return gerror.New("插件不存在")
	}
	if entry.Status != StatusEnabled {
		return gerror.New("插件未启用")
	}

	if err := entry.Plugin.Destroy(ctx); err != nil {
		glog.Warningf(ctx, "plugin %s: destroy error during disable: %v", name, err)
	}

	app.Hook.RemoveHooks(name)

	entry.Status = StatusDisabled
	_, err := dao.SysPlugins.Ctx(ctx).
		Where("name", name).
		Data(do.SysPlugins{Status: StatusDisabled}).
		Update()
	return err
}

// Uninstall 卸载插件。
func Uninstall(ctx context.Context, name string, keepData bool) error {
	entry := GetPlugin(name)
	if entry == nil {
		return gerror.New("插件不存在")
	}
	if entry.Status == StatusEnabled {
		return gerror.New("请先禁用插件再卸载")
	}

	if !keepData {
		if err := entry.Plugin.Uninstall(ctx); err != nil {
			glog.Warningf(ctx, "plugin %s: uninstall cleanup failed: %v", name, err)
		}
	}

	_, err := dao.SysPlugins.Ctx(ctx).Where("name", name).Delete()
	if err != nil {
		return err
	}

	_, err = dao.TntTenantPlugins.Ctx(ctx).Where("plugin_name", name).Delete()
	if err != nil {
		return err
	}

	entry.Status = StatusRegistered
	glog.Infof(ctx, "plugin %s: uninstalled (keepData=%v)", name, keepData)
	return nil
}

// Upgrade 升级插件。
func Upgrade(ctx context.Context, name string) error {
	entry := GetPlugin(name)
	if entry == nil {
		return gerror.New("插件不存在")
	}

	if err := entry.Plugin.Upgrade(ctx); err != nil {
		return gerror.Wrap(err, "插件升级失败")
	}

	_, err := dao.SysPlugins.Ctx(ctx).
		Where("name", name).
		Data(do.SysPlugins{Version: entry.Info.Version}).
		Update()
	return err
}

// EnableForTenant 为指定租户启用插件。
func EnableForTenant(ctx context.Context, pluginName string, tenantID int64) error {
	entry := GetPlugin(pluginName)
	if entry == nil {
		return gerror.New("插件不存在")
	}
	if entry.Status != StatusEnabled {
		return gerror.New("插件未全局启用")
	}

	if ta, ok := entry.Plugin.(TenantAware); ok {
		if err := ta.OnTenantEnable(ctx, tenantID); err != nil {
			return gerror.Wrap(err, "租户级插件启用失败")
		}
	}

	_, err := dao.TntTenantPlugins.Ctx(ctx).
		Data(do.TntTenantPlugins{
			TenantId:   tenantID,
			PluginName: pluginName,
			Enabled:    true,
		}).
		OnConflict("tenant_id,plugin_name").
		Save()
	return err
}

// DisableForTenant 为指定租户禁用插件。
func DisableForTenant(ctx context.Context, pluginName string, tenantID int64) error {
	entry := GetPlugin(pluginName)
	if entry == nil {
		return gerror.New("插件不存在")
	}

	if ta, ok := entry.Plugin.(TenantAware); ok {
		if err := ta.OnTenantDisable(ctx, tenantID); err != nil {
			glog.Warningf(ctx, "plugin %s: OnTenantDisable failed for tenant %d: %v", pluginName, tenantID, err)
		}
	}

	_, err := dao.TntTenantPlugins.Ctx(ctx).
		Data(do.TntTenantPlugins{Enabled: false}).
		Where("tenant_id = ? AND plugin_name = ?", tenantID, pluginName).
		Update()
	return err
}

// --- 内部辅助函数 ---

func checkDependencies(entry *PluginEntry) error {
	for _, dep := range entry.Info.Dependencies {
		depEntry := GetPlugin(dep)
		if depEntry == nil {
			return gerror.Newf("依赖插件 %s 不存在", dep)
		}
		if depEntry.Status != StatusEnabled && depEntry.Status != StatusInstalled {
			return gerror.Newf("依赖插件 %s 未安装", dep)
		}
	}
	return nil
}

func registerConfigSchema(ctx context.Context, pluginName string, fields []ConfigFieldDef) error {
	for _, field := range fields {
		key := fmt.Sprintf("plugin.%s.%s", pluginName, field.Key)
		_, err := dao.SysOptions.Ctx(ctx).
			Data(do.SysOptions{
				Key:         key,
				Value:       fmt.Sprintf("%v", field.Default),
				Description: field.Description,
			}).
			OnConflict("key").
			Save()
		if err != nil {
			return err
		}
	}
	return nil
}
