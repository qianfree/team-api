package admin

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/plugin"
)

// permMergePlugin 测试用最小插件骨架（logic/admin 侧只关心权限声明）。
type permMergePlugin struct {
	name   string
	groups []v1.PermissionGroup
	mod    []v1.PermissionModuleMeta
}

var permMergeSeq int64

func newPermMergePlugin(t *testing.T, module string) *plugin.PluginEntry {
	t.Helper()
	name := "perm-merge-" + strings.ToLower(strings.NewReplacer("/", "-", "_", "-").Replace(t.Name())) +
		"-" + string(rune('a'+atomic.AddInt64(&permMergeSeq, 1)%26))
	p := &permMergePlugin{
		name:   name,
		groups: []v1.PermissionGroup{{Name: module, Label: module, Permissions: []string{module + ":view"}}},
		mod: []v1.PermissionModuleMeta{{
			Module: module,
			Label:  module,
			Tiers:  []v1.TierOption{{Tier: TierRead, Label: "只读", Permissions: []string{module + ":view"}}},
		}},
	}
	plugin.Register(p)
	entry := plugin.GetPlugin(name)
	t.Cleanup(func() { entry.Status = plugin.StatusRegistered })
	return entry
}

func (p *permMergePlugin) Info() plugin.PluginInfo {
	return plugin.PluginInfo{Name: p.name, Label: p.name, Version: "1.0.0"}
}
func (p *permMergePlugin) Init(ctx context.Context, app *plugin.App) error { return nil }
func (p *permMergePlugin) Destroy(ctx context.Context) error               { return nil }
func (p *permMergePlugin) Install(ctx context.Context) error               { return nil }
func (p *permMergePlugin) Upgrade(ctx context.Context) error               { return nil }
func (p *permMergePlugin) Uninstall(ctx context.Context) error             { return nil }
func (p *permMergePlugin) PermissionGroups() []v1.PermissionGroup          { return p.groups }
func (p *permMergePlugin) PermissionModules() []v1.PermissionModuleMeta    { return p.mod }
func (p *permMergePlugin) PermissionRules() []plugin.RoutePermRule         { return nil }

// TestBuildValidPermissionSetMergesPluginPoints 已注册（未安装/未启用）插件声明的权限点
// 必须是有效权限点——否则插件禁用后，仍持有其权限点的角色在下一次保存时会被
// sanitizeGrantedPermissions 判为「无效的权限点」而无法保存。
func TestBuildValidPermissionSetMergesPluginPoints(t *testing.T) {
	newPermMergePlugin(t, "mergeval") // 状态保持 registered

	set := buildValidPermissionSet()
	if !set["mergeval:view"] {
		t.Fatal("已注册插件的权限点应进入有效权限点集合（不限安装状态）")
	}
}

// TestAllPermissionPointsIncludesPluginPoints 超管全量权限集与有效集合同口径。
func TestAllPermissionPointsIncludesPluginPoints(t *testing.T) {
	newPermMergePlugin(t, "mergeall")

	found := false
	for _, p := range AllPermissionPoints() {
		if p == "mergeall:view" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("已注册插件的权限点应出现在 AllPermissionPoints")
	}
}

// TestGetAllPermissionsMergesEnabledPluginModules 角色配置元数据只合并【已启用】插件：
// 未启用插件的模块不占用授权界面，启用后出现。
func TestGetAllPermissionsMergesEnabledPluginModules(t *testing.T) {
	entry := newPermMergePlugin(t, "mergemod")
	s := &sAdmin{}

	// 未启用：分组与模块均不出现
	res, err := s.GetAllPermissions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetAllPermissions 失败: %v", err)
	}
	for _, g := range res.Groups {
		if g.Name == "mergemod" {
			t.Fatal("未启用插件的分组不应出现在 Groups")
		}
	}
	for _, m := range res.Modules {
		if m.Module == "mergemod" {
			t.Fatal("未启用插件的模块不应出现在 Modules")
		}
	}

	// 启用后出现
	entry.Status = plugin.StatusEnabled
	res, err = s.GetAllPermissions(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetAllPermissions 失败: %v", err)
	}
	groupFound, moduleFound := false, false
	for _, g := range res.Groups {
		if g.Name == "mergemod" {
			groupFound = true
		}
	}
	for _, m := range res.Modules {
		if m.Module == "mergemod" {
			moduleFound = true
		}
	}
	if !groupFound || !moduleFound {
		t.Fatalf("启用插件的分组(%v)与模块(%v)都应出现", groupFound, moduleFound)
	}
}
