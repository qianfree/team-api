package middleware

import (
	"strings"
	"sync/atomic"
	"testing"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/plugin"
)

// dynRuleProvider 测试用 PermissionProvider 实现：只声明规则与分组，
// 不注册成完整插件（SetPermissionRules 直接消费 provider，无需走生命周期）。
type dynRuleProvider struct {
	groups []v1.PermissionGroup
	rules  []plugin.RoutePermRule
}

func (p *dynRuleProvider) PermissionGroups() []v1.PermissionGroup       { return p.groups }
func (p *dynRuleProvider) PermissionModules() []v1.PermissionModuleMeta { return nil }
func (p *dynRuleProvider) PermissionRules() []plugin.RoutePermRule      { return p.rules }

var dynTestSeq int64

// registerDynRules 用唯一 owner 名登记一组动态规则并在测试结束后摘除，
// 避免全局注册表在包内用例间串扰。
func registerDynRules(t *testing.T, groups []v1.PermissionGroup, rules []plugin.RoutePermRule) string {
	t.Helper()
	owner := "dyn-test-" + strings.ToLower(strings.NewReplacer("/", "-", "_", "-").Replace(t.Name())) +
		"-" + string(rune('a'+atomic.AddInt64(&dynTestSeq, 1)%26))
	plugin.SetPermissionRules(owner, &dynRuleProvider{groups: groups, rules: rules})
	t.Cleanup(func() { plugin.ClearPermissionRules(owner) })
	return owner
}

// TestDynamicRuleLongerPrefixBeatsStaticPluginRule 是本改造的核心语义：
// 插件功能路由前缀比通用插件管理静态规则（/api/admin/plugins/ → system:plugin）更长，
// 合并匹配时必须胜出，否则插件权限声明形同虚设。
func TestDynamicRuleLongerPrefixBeatsStaticPluginRule(t *testing.T) {
	registerDynRules(t,
		[]v1.PermissionGroup{{Name: "reqguard", Permissions: []string{"reqguard:view"}}},
		[]plugin.RoutePermRule{
			{Method: "GET", Prefix: "/api/admin/plugins/reqguard/", Perm: "reqguard:view"},
		})

	if got := matchPermission("GET", "/api/admin/plugins/reqguard/reports"); got != "reqguard:view" {
		t.Fatalf("插件路由应命中动态规则 reqguard:view，实际 %q", got)
	}

	// 通用插件管理路径不受影响，仍走静态规则
	if got := matchPermission("GET", "/api/admin/plugins"); got != "system:plugin" {
		t.Fatalf("通用插件列表应仍命中静态 system:plugin，实际 %q", got)
	}
	// 未声明动态规则的其他插件路径回落到静态通用前缀
	if got := matchPermission("GET", "/api/admin/plugins/other/config"); got != "system:plugin" {
		t.Fatalf("未登记插件的路径应回落到静态 system:plugin，实际 %q", got)
	}
}

// TestDynamicExactPathBeatsStaticPrefix 阶段优先级跨来源保持：
// 动态精确规则（阶段1）优先于静态前缀规则（阶段3）。
func TestDynamicExactPathBeatsStaticPrefix(t *testing.T) {
	registerDynRules(t,
		[]v1.PermissionGroup{{Name: "reqguard", Permissions: []string{"reqguard:config"}}},
		[]plugin.RoutePermRule{
			{Method: "GET", Path: "/api/admin/plugins/reqguard/ai/test", Perm: "reqguard:config"},
		})

	if got := matchPermission("GET", "/api/admin/plugins/reqguard/ai/test"); got != "reqguard:config" {
		t.Fatalf("动态精确规则应胜出，实际 %q", got)
	}
	// 同前缀其余路径仍走静态兜底
	if got := matchPermission("GET", "/api/admin/plugins/reqguard/other"); got != "system:plugin" {
		t.Fatalf("未精确命中的路径应回落静态规则，实际 %q", got)
	}
}

// TestDynamicRulesMethodMismatch 方法不匹配的动态规则不命中。
func TestDynamicRulesMethodMismatch(t *testing.T) {
	registerDynRules(t,
		[]v1.PermissionGroup{{Name: "reqguard", Permissions: []string{"reqguard:analyze"}}},
		[]plugin.RoutePermRule{
			{Method: "POST", Prefix: "/api/admin/plugins/reqguard/analyze", Perm: "reqguard:analyze"},
		})

	if got := matchPermission("GET", "/api/admin/plugins/reqguard/analyze"); got != "system:plugin" {
		t.Fatalf("方法不匹配不应命中动态规则，应回落静态 system:plugin，实际 %q", got)
	}
}

// TestDynamicRulesClearedFallBack 规则摘除后（插件禁用）回落到静态匹配。
func TestDynamicRulesClearedFallBack(t *testing.T) {
	owner := registerDynRules(t,
		[]v1.PermissionGroup{{Name: "reqguard", Permissions: []string{"reqguard:view"}}},
		[]plugin.RoutePermRule{
			{Method: "GET", Prefix: "/api/admin/plugins/reqguard/", Perm: "reqguard:view"},
		})
	if got := matchPermission("GET", "/api/admin/plugins/reqguard/reports"); got != "reqguard:view" {
		t.Fatalf("前置校验失败：登记后应命中动态规则，实际 %q", got)
	}

	plugin.ClearPermissionRules(owner)
	if got := matchPermission("GET", "/api/admin/plugins/reqguard/reports"); got != "system:plugin" {
		t.Fatalf("摘除后应回落静态 system:plugin，实际 %q", got)
	}
}

// TestNoDynamicRulesFastPath 无动态规则时匹配行为与改造前完全一致。
func TestNoDynamicRulesFastPath(t *testing.T) {
	if got := matchPermission("GET", "/api/admin/tenants"); got != "tenant:view" {
		t.Fatalf("静态精确匹配失效：%q", got)
	}
	if got := matchPermission("GET", "/api/admin/unknown-module/xxx"); got != "" {
		t.Fatalf("未知路径应默认拒绝（空权限），实际 %q", got)
	}
}
