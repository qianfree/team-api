package plugin

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// permTestPlugin 测试用最小插件骨架：只实现核心接口 + PermissionProvider。
type permTestPlugin struct {
	name   string
	groups []v1.PermissionGroup
	mod    []v1.PermissionModuleMeta
	rules  []RoutePermRule
}

var permTestSeq int64

// newPermTestPlugin 注册一个带唯一名字的测试插件并返回其条目。
// 注册表是进程级全局且无反注册，用自增序号避免用例间重名 panic。
func newPermTestPlugin(t *testing.T, groups []v1.PermissionGroup, rules []RoutePermRule) *PluginEntry {
	t.Helper()
	name := "perm-test-" + strings.ToLower(strings.NewReplacer("/", "-", "_", "-").Replace(t.Name())) +
		"-" + string(rune('a'+atomic.AddInt64(&permTestSeq, 1)%26))
	p := &permTestPlugin{name: name, groups: groups, rules: rules}
	Register(p)
	return GetPlugin(name)
}

func (p *permTestPlugin) Info() PluginInfo {
	return PluginInfo{Name: p.name, Label: p.name, Version: "1.0.0"}
}
func (p *permTestPlugin) Init(ctx context.Context, app *App) error     { return nil }
func (p *permTestPlugin) Destroy(ctx context.Context) error            { return nil }
func (p *permTestPlugin) Install(ctx context.Context) error            { return nil }
func (p *permTestPlugin) Upgrade(ctx context.Context) error            { return nil }
func (p *permTestPlugin) Uninstall(ctx context.Context) error          { return nil }
func (p *permTestPlugin) PermissionGroups() []v1.PermissionGroup       { return p.groups }
func (p *permTestPlugin) PermissionModules() []v1.PermissionModuleMeta { return p.mod }
func (p *permTestPlugin) PermissionRules() []RoutePermRule             { return p.rules }

func testGroups(name string) []v1.PermissionGroup {
	return []v1.PermissionGroup{{
		Name:        name,
		Label:       name,
		Permissions: []string{name + ":view", name + ":edit"},
	}}
}

// TestSetPermissionRulesValidation 验证登记时的防御性校验：
// 引用未声明权限点、path/prefix 缺一或同时给出、方法缺失的规则被丢弃，
// 小写方法被归一化为大写。
func TestSetPermissionRulesValidation(t *testing.T) {
	entry := newPermTestPlugin(t, testGroups("valmod"), []RoutePermRule{
		{Method: "GET", Prefix: "/api/admin/plugins/valmod/", Perm: "valmod:view"},        // 合法
		{Method: "GET", Prefix: "/api/admin/plugins/valmod/x", Perm: "valmod:unknown"},    // 未声明的权限点
		{Method: "GET", Path: "/a", Prefix: "/b", Perm: "valmod:view"},                    // path 与 prefix 同时给出
		{Method: "GET", Perm: "valmod:view"},                                              // path 与 prefix 均缺失
		{Method: "", Prefix: "/api/admin/plugins/valmod/y", Perm: "valmod:view"},          // 方法缺失
		{Method: "delete", Prefix: "/api/admin/plugins/valmod/del/", Perm: "valmod:edit"}, // 小写方法归一化
	})
	defer ClearPermissionRules(entry.Info.Name)

	SetPermissionRules(entry.Info.Name, entry.Plugin.(PermissionProvider))

	rules := DynamicPermissionRules()
	var got []RoutePermRule
	for _, r := range rules {
		if r.Perm == "valmod:view" || r.Perm == "valmod:edit" {
			if strings.HasPrefix(r.Prefix+r.Path, "/api/admin/plugins/valmod") {
				got = append(got, r)
			}
		}
	}
	if len(got) != 2 {
		t.Fatalf("应保留 2 条合法规则，实际 %d 条：%+v", len(got), got)
	}
	for _, r := range got {
		if r.Method != strings.ToUpper(r.Method) {
			t.Errorf("方法未归一化为大写：%q", r.Method)
		}
	}
}

// TestRegisteredPermissionGroupsIgnoresStatus 已注册（未安装/未启用）插件的权限点
// 也必须进入有效权限点集合——否则插件禁用后角色保存会被判无效权限点。
func TestRegisteredPermissionGroupsIgnoresStatus(t *testing.T) {
	entry := newPermTestPlugin(t, testGroups("regmod"), nil)
	// 状态保持 StatusRegistered（未安装）
	if entry.Status != StatusRegistered {
		t.Fatalf("新注册插件状态应为 registered，实际 %s", entry.Status)
	}

	found := false
	for _, g := range RegisteredPermissionGroups() {
		if g.Name == "regmod" {
			found = true
		}
	}
	if !found {
		t.Fatal("已注册插件的权限分组应出现在 RegisteredPermissionGroups（不限安装状态）")
	}
}

// TestEnabledPermissionFilters 未启用插件的分组/模块不进入启用集合，启用后进入。
func TestEnabledPermissionFilters(t *testing.T) {
	entry := newPermTestPlugin(t, testGroups("enmod"), nil)
	entry.Plugin.(*permTestPlugin).mod = []v1.PermissionModuleMeta{{
		Module: "enmod",
		Label:  "enmod",
		Tiers:  []v1.TierOption{{Tier: "read", Label: "只读", Permissions: []string{"enmod:view"}}},
	}}

	if len(EnabledPermissionGroups()) != 0 || len(EnabledPermissionModules()) != 0 {
		t.Fatal("未启用插件不应出现在 Enabled 集合")
	}

	entry.Status = StatusEnabled
	defer func() { entry.Status = StatusRegistered }()

	if len(EnabledPermissionGroups()) == 0 || len(EnabledPermissionModules()) == 0 {
		t.Fatal("启用插件的分组与模块应出现在 Enabled 集合")
	}
}

// TestClearPermissionRules 摘除后动态规则集合不再包含该插件条目。
func TestClearPermissionRules(t *testing.T) {
	entry := newPermTestPlugin(t, testGroups("clrmod"), []RoutePermRule{
		{Method: "GET", Prefix: "/api/admin/plugins/clrmod/", Perm: "clrmod:view"},
	})
	SetPermissionRules(entry.Info.Name, entry.Plugin.(PermissionProvider))
	if len(DynamicPermissionRules()) == 0 {
		t.Fatal("登记后动态规则不应为空")
	}

	ClearPermissionRules(entry.Info.Name)
	for _, r := range DynamicPermissionRules() {
		if r.Prefix == "/api/admin/plugins/clrmod/" {
			t.Fatal("摘除后不应残留该插件的规则")
		}
	}
}
