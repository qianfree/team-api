package example

import (
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/plugin"
)

// PermissionGroups 实现 PermissionProvider 接口 —— 声明插件自己的权限点。
//
// 不实现该接口的插件路由会被 AdminPermissionGuard 默认拒绝（仅 super_admin 可用）；
// 实现后权限点可被授予角色，插件启用时模块自动出现在角色配置界面（前端零改动）。
func (p *examplePlugin) PermissionGroups() []v1.PermissionGroup {
	return []v1.PermissionGroup{
		{
			Name:  "example",
			Label: "示例插件",
			Permissions: []string{
				"example:view", "example:edit",
			},
		},
	}
}

// PermissionModules 实现「模块 × 档位」元数据，Module 必须与 PermissionGroups 的 Name 对应。
// 档位值须为核心档位常量（none/read/operate/full），高档位应包含低档位全部权限点。
func (p *examplePlugin) PermissionModules() []v1.PermissionModuleMeta {
	return []v1.PermissionModuleMeta{
		{
			Module: "example",
			Label:  "示例插件",
			Tiers: []v1.TierOption{
				{Tier: "read", Label: "只读", Permissions: []string{"example:view"}},
				{Tier: "operate", Label: "操作", Permissions: []string{"example:view", "example:edit"}},
			},
		},
	}
}

// PermissionRules 声明插件路由的权限映射，与 Routes() 注册的路由一一对应。
// 前缀必须落在插件自己的命名空间（/api/admin/plugins/{插件名}/）下，
// 更长的前缀会在 guard 匹配时胜过通用的插件管理规则（system:plugin）。
func (p *examplePlugin) PermissionRules() []plugin.RoutePermRule {
	return []plugin.RoutePermRule{
		{Method: "GET", Prefix: "/api/admin/plugins/example/", Perm: "example:view"},
		{Method: "POST", Prefix: "/api/admin/plugins/example/", Perm: "example:edit"},
	}
}
