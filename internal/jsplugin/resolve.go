package jsplugin

import (
	"strings"

	"github.com/qianfree/team-api/internal/jsplugin/contract"
)

// JSPlatformPrefix 任务平台标识前缀（task_handler 写入 task.Platform，jsbridge 解析）。
const JSPlatformPrefix = "js:"

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
