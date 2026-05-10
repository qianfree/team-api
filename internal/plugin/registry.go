package plugin

import (
	"context"
	"sync"

	"github.com/gogf/gf/v2/os/glog"
)

var (
	plugins = make(map[string]*PluginEntry)
	mu      sync.RWMutex
	app     *App
)

// PluginEntry 插件注册条目。
type PluginEntry struct {
	Plugin Plugin
	Info   PluginInfo
	Status PluginStatus
}

// PluginStatus 插件状态类型。
type PluginStatus string

const (
	StatusRegistered PluginStatus = "registered" // 已注册（代码加载但未安装）
	StatusInstalled  PluginStatus = "installed"  // 已安装（数据库初始化完成）
	StatusEnabled    PluginStatus = "enabled"    // 已启用（正在运行）
	StatusDisabled   PluginStatus = "disabled"   // 已禁用（已安装但未运行）
	StatusError      PluginStatus = "error"      // 异常
)

// Register 注册插件（在插件的 init() 函数中调用）。
func Register(p Plugin) {
	mu.Lock()
	defer mu.Unlock()

	info := p.Info()
	if info.Name == "" {
		panic("plugin: name is required")
	}
	if _, exists := plugins[info.Name]; exists {
		panic("plugin: duplicate registration: " + info.Name)
	}

	plugins[info.Name] = &PluginEntry{
		Plugin: p,
		Info:   info,
		Status: StatusRegistered,
	}

	glog.Infof(context.Background(), "plugin registered: %s v%s", info.Name, info.Version)
}

// GetPlugin 获取已注册的插件。
func GetPlugin(name string) *PluginEntry {
	mu.RLock()
	defer mu.RUnlock()
	return plugins[name]
}

// AllPlugins 获取所有已注册插件。
func AllPlugins() []*PluginEntry {
	mu.RLock()
	defer mu.RUnlock()
	result := make([]*PluginEntry, 0, len(plugins))
	for _, entry := range plugins {
		result = append(result, entry)
	}
	return result
}

// GetApp 获取框架服务实例。
func GetApp() *App {
	return app
}
