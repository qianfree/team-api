package jsplugin

import (
	"strings"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// JSPlatformPrefix 任务平台标识前缀（task_handler 写入 task.Platform，jsbridge 解析）。
const JSPlatformPrefix = "js:"

// ResolvePluginKeyForModel 按模型名解析接管的插件 key（大小写不敏感）。
// v1 冲突预检保证模型对插件独占，故「模型 → 插件」是可靠路由——渠道无需
// 显式配置 plugin_key（settings.plugin_key 为可选覆盖，配置时必须一致）。
func ResolvePluginKeyForModel(model string) (string, bool) {
	return CurrentGeneration().FindPluginByModel(model)
}

// PlatformForKey 插件 key → 任务平台标识（js:{key}）。
func PlatformForKey(pluginKey string) string {
	return JSPlatformPrefix + pluginKey
}

// KeyFromPlatform 任务平台标识 → 插件 key（非 js 前缀返回空串）。
func KeyFromPlatform(platform string) string {
	return strings.TrimPrefix(platform, JSPlatformPrefix)
}

// compile guard：防止 contract 导入被误删（Phase 2 的 ActiveManifest 使用）。
var _ = contract.APIVersion
