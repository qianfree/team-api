package plugin

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/database/gredis"
	"github.com/gogf/gf/v2/net/ghttp"
)

// Plugin 插件核心接口，所有插件必须实现。
type Plugin interface {
	// Info 返回插件元信息（名称、版本、描述等）。
	Info() PluginInfo
	// Init 初始化插件，app 提供框架服务访问入口（服务启动时调用）。
	Init(ctx context.Context, app *App) error
	// Destroy 销毁插件，释放资源（服务关闭或插件禁用时调用）。
	Destroy(ctx context.Context) error
	// Install 安装插件（管理后台点击"安装"时调用，建表、初始化数据）。
	Install(ctx context.Context) error
	// Upgrade 升级插件（插件版本变更时调用，执行增量迁移）。
	Upgrade(ctx context.Context) error
	// Uninstall 卸载插件（管理后台点击"卸载"时调用，清理数据）。
	Uninstall(ctx context.Context) error
}

// PluginInfo 插件元信息。
type PluginInfo struct {
	Name         string   `json:"name"`         // 唯一标识，如 "email-report"
	Label        string   `json:"label"`        // 显示名称，如 "邮件报表"
	Description  string   `json:"description"`  // 功能描述
	Author       string   `json:"author"`       // 作者
	Version      string   `json:"version"`      // 语义化版本，如 "1.0.0"
	Category     string   `json:"category"`     // 分类：relay/middleware/billing/notification/extension
	Dependencies []string `json:"dependencies"` // 依赖的其他插件
}

// App 插件访问框架服务的推荐入口。
type App struct {
	Server *ghttp.Server // HTTP 服务器
	DB     gdb.DB        // 数据库
	Redis  *gredis.Redis // Redis
	Hook   *HookEmitter  // 事件发射器
}
