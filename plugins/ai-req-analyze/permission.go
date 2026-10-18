package aireqanalyze

import (
	v1 "github.com/qianfree/team-api/api/admin/v1"

	"github.com/qianfree/team-api/internal/plugin"
)

// 权限点统一从 PluginName 派生，标识变更时零漂移。
const (
	PermView    = PluginName + ":view"    // 查看报告列表/详情、巡查记录、任务状态
	PermAnalyze = PluginName + ":analyze" // 发起按需分析、查看证据明细、删除报告
	PermConfig  = PluginName + ":config"  // 配置分析 AI 连接、分析参数、巡查计划
)

// 路由命名空间：所有功能端点挂在此前缀下，比通用插件管理静态规则
// （/api/admin/plugins/ → system:plugin）更长，guard 匹配时胜出。
const routeBase = "/api/admin/plugins/" + PluginName

// PermissionGroups 实现 PermissionProvider 接口。
func (p *aiReqAnalyzePlugin) PermissionGroups() []v1.PermissionGroup {
	return []v1.PermissionGroup{
		{
			Name:  PluginName,
			Label: "AI请求分析",
			Permissions: []string{
				PermView,
				PermAnalyze,
				PermConfig,
			},
		},
	}
}

// PermissionModules 实现「模块 × 档位」元数据：
// 只读=查看报告；操作=+发起分析；完全=+插件自身配置。
func (p *aiReqAnalyzePlugin) PermissionModules() []v1.PermissionModuleMeta {
	return []v1.PermissionModuleMeta{
		{
			Module: PluginName,
			Label:  "AI请求分析",
			Tiers: []v1.TierOption{
				{Tier: "read", Label: "只读", Permissions: []string{PermView}},
				{Tier: "operate", Label: "操作", Permissions: []string{PermView, PermAnalyze}},
				{Tier: "full", Label: "完全", Permissions: []string{PermView, PermAnalyze, PermConfig}},
			},
		},
	}
}

// PermissionRules 声明全部功能端点的权限映射。
func (p *aiReqAnalyzePlugin) PermissionRules() []plugin.RoutePermRule {
	return []plugin.RoutePermRule{
		// 发起按需分析与删除报告是写操作，挂操作档权限点
		{Method: "POST", Path: routeBase + "/analyze", Perm: PermAnalyze},
		{Method: "DELETE", Prefix: routeBase + "/reports/", Perm: PermAnalyze},

		// 报告列表/详情/会话还原/任务列表均为只读
		//（前缀不带尾斜杠，同时覆盖 /reports 与 /reports/{id}[/conversation]）
		{Method: "GET", Prefix: routeBase + "/reports", Perm: PermView},
		{Method: "GET", Path: routeBase + "/jobs", Perm: PermView},

		// 连通性测试会真实调用外部 AI 端点并验证密钥，属配置能力
		{Method: "GET", Path: routeBase + "/ai/test", Perm: PermConfig},
	}
}
