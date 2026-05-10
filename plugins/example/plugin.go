package example

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/os/glog"

	"github.com/qianfree/team-api/internal/plugin"
	"github.com/qianfree/team-api/plugins/example/hooks"
)

type examplePlugin struct{}

func init() {
	plugin.Register(&examplePlugin{})
}

// Info 实现 Plugin 基础接口
func (p *examplePlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{
		Name:        "example",
		Label:       "功能示例",
		Description: "插件系统功能示例，展示所有扩展接口的用法",
		Author:      "team-api",
		Version:     "1.0.0",
		Category:    "extension",
	}
}

// Init 实现 Plugin 基础接口
func (p *examplePlugin) Init(ctx context.Context, app *plugin.App) error {
	glog.Info(ctx, "example plugin initialized")
	return nil
}

// Destroy 实现 Plugin 基础接口
func (p *examplePlugin) Destroy(ctx context.Context) error {
	glog.Info(ctx, "example plugin destroyed")
	return nil
}

// Install 建表
func (p *examplePlugin) Install(ctx context.Context) error {
	_, err := g.DB().Exec(ctx, `
		CREATE TABLE IF NOT EXISTS plg_example_logs (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT       NOT NULL,
			action     VARCHAR(64)  NOT NULL,
			detail     TEXT         DEFAULT '',
			created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
		);
		COMMENT ON TABLE plg_example_logs IS '示例插件-操作日志';
		CREATE INDEX IF NOT EXISTS idx_plg_example_logs_tenant_id ON plg_example_logs (tenant_id);
		CREATE INDEX IF NOT EXISTS idx_plg_example_logs_created_at ON plg_example_logs USING BRIN (created_at);
	`)
	return err
}

// Upgrade 增量迁移
func (p *examplePlugin) Upgrade(ctx context.Context) error {
	return nil
}

// Uninstall 清理数据
func (p *examplePlugin) Uninstall(ctx context.Context) error {
	_, err := g.DB().Exec(ctx, `DROP TABLE IF EXISTS plg_example_logs`)
	return err
}

// --- 可选接口实现 ---

// Hooks 实现 Hookable 接口
func (p *examplePlugin) Hooks() []plugin.HookBinding {
	return []plugin.HookBinding{
		{
			Event:    plugin.HookRelayBeforeRequest,
			Priority: 100,
			Handler:  hooks.RelayBeforeRequest,
		},
		{
			Event:    plugin.HookRelayAfterResponse,
			Priority: 100,
			Handler:  hooks.RelayAfterResponse,
		},
	}
}

// Routes 实现 Routable 接口
func (p *examplePlugin) Routes(ctx context.Context, server *ghttp.Server) error {
	// 示例：注册插件自定义路由
	// 实际使用时可以绑定到 admin 或 tenant 路由组
	return nil
}

// CronJobs 实现 Cronable 接口
func (p *examplePlugin) CronJobs() []plugin.CronJobDef {
	return []plugin.CronJobDef{
		{
			Name:      "example_daily_cleanup",
			CronExpr:  "0 3 * * *",
			Handler:   p.dailyCleanup,
			Singleton: true,
		},
	}
}

func (p *examplePlugin) dailyCleanup(ctx context.Context) {
	glog.Info(ctx, "[example plugin] running daily cleanup")
	_, err := g.DB().Exec(ctx, fmt.Sprintf(
		`DELETE FROM plg_example_logs WHERE created_at < NOW() - INTERVAL '%d days'`, 30,
	))
	if err != nil {
		glog.Warningf(ctx, "[example plugin] cleanup failed: %v", err)
	}
}

// ConfigSchema 实现 Configurable 接口
func (p *examplePlugin) ConfigSchema() []plugin.ConfigFieldDef {
	return []plugin.ConfigFieldDef{
		{
			Key:         "max_log_retention_days",
			Label:       "日志保留天数",
			Type:        "int",
			Default:     30,
			Required:    false,
			Description: "操作日志保留天数，超过此天数自动清理",
		},
		{
			Key:         "enable_detailed_logging",
			Label:       "详细日志",
			Type:        "bool",
			Default:     false,
			Required:    false,
			Description: "是否记录详细的请求/响应内容",
		},
		{
			Key:         "log_level",
			Label:       "日志级别",
			Type:        "select",
			Default:     "info",
			Options:     []string{"debug", "info", "warn", "error"},
			Required:    false,
			Description: "插件日志级别",
		},
	}
}

// OnTenantEnable 实现 TenantAware 接口
func (p *examplePlugin) OnTenantEnable(ctx context.Context, tenantID int64) error {
	glog.Infof(ctx, "[example plugin] enabled for tenant %d", tenantID)
	return nil
}

// OnTenantDisable 实现 TenantAware 接口
func (p *examplePlugin) OnTenantDisable(ctx context.Context, tenantID int64) error {
	glog.Infof(ctx, "[example plugin] disabled for tenant %d", tenantID)
	return nil
}
