package plugin

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/logic/common"
)

// Bootstrap 启动插件系统（在 cmd.go 的服务启动流程中调用）。
func Bootstrap(ctx context.Context, appInstance *App) error {
	app = appInstance

	entries := AllPlugins()
	glog.Infof(ctx, "plugin system bootstrapping, %d plugin(s) registered", len(entries))

	for _, entry := range entries {
		// 从数据库读取安装状态
		dbStatus, err := getPluginStatusFromDB(ctx, entry.Info.Name)
		if err != nil {
			glog.Warningf(ctx, "plugin %s: failed to read db status: %v", entry.Info.Name, err)
			continue
		}

		// 数据库中没有记录，说明是新注册但未安装的插件
		if dbStatus == "" {
			entry.Status = StatusRegistered
			continue
		}

		entry.Status = dbStatus

		// 只初始化和启动已启用的插件
		if entry.Status != StatusEnabled {
			continue
		}

		if err := startPlugin(ctx, entry); err != nil {
			entry.Status = StatusError
			glog.Errorf(ctx, "plugin %s: start failed: %v", entry.Info.Name, err)
		}
	}

	return nil
}

// startPlugin 启动单个插件（初始化 + 注册 Hook/定时任务）。
func startPlugin(ctx context.Context, entry *PluginEntry) error {
	// 1. 调用插件初始化
	if err := entry.Plugin.Init(ctx, app); err != nil {
		return err
	}

	// 2. 注册 Hook（如果实现了 Hookable）
	if h, ok := entry.Plugin.(Hookable); ok {
		for _, binding := range h.Hooks() {
			app.Hook.RegisterHook(entry.Info.Name, binding.Event, binding.Priority, binding.Handler)
		}
	}

	// 3. 注册定时任务（如果实现了 Cronable）
	if c, ok := entry.Plugin.(Cronable); ok {
		cs := common.GetCronScheduler()
		for _, job := range c.CronJobs() {
			jobFunc := job.Handler
			jobName := "plugin_" + job.Name
			cs.Register(jobName, job.CronExpr, func(ctx context.Context) error {
				jobFunc(ctx)
				return nil
			})
		}
	}

	entry.Status = StatusEnabled
	glog.Infof(ctx, "plugin %s: started successfully", entry.Info.Name)
	return nil
}

// RegisterAllRoutes 注册所有已启用插件的路由。
func RegisterAllRoutes(ctx context.Context, server *ghttp.Server) {
	for _, entry := range AllPlugins() {
		if entry.Status != StatusEnabled {
			continue
		}
		if r, ok := entry.Plugin.(Routable); ok {
			if err := r.Routes(ctx, server); err != nil {
				glog.Errorf(ctx, "plugin %s: route registration failed: %v", entry.Info.Name, err)
			}
		}
	}
}

// Shutdown 关闭所有插件（在服务关闭时调用）。
func Shutdown(ctx context.Context) {
	for _, entry := range AllPlugins() {
		if entry.Status == StatusEnabled {
			if err := entry.Plugin.Destroy(ctx); err != nil {
				glog.Warningf(ctx, "plugin %s: destroy failed: %v", entry.Info.Name, err)
			}
		}
	}
	glog.Info(ctx, "plugin system shutdown complete")
}

// getPluginStatusFromDB 从数据库读取插件状态。
func getPluginStatusFromDB(ctx context.Context, name string) (PluginStatus, error) {
	record, err := g.DB().Model("sys_plugins").Ctx(ctx).Where("name", name).One()
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", nil
	}
	return PluginStatus(record["status"].String()), nil
}
