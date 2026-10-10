package plugin

import (
	"context"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/os/glog"

	v1 "github.com/qianfree/team-api/api/admin/v1"
)

// 插件权限声明的登记与收集。
//
// 依赖方向说明（为什么注册表在本包而不是 middleware）：AdminPermissionGuard 依赖
// logic/admin（HasPermission），logic/admin 又依赖本包收集权限点；若本包为登记规则
// 再依赖 middleware，三条依赖即成环。因此动态规则注册表放在 plugin 包内，
// middleware 反向只读查询（DynamicPermissionRules），登记/摘除由本包生命周期函数完成。
var (
	permRulesMu    sync.RWMutex
	permRuleOwners = make(map[string][]RoutePermRule) // 插件名 → 已登记规则
)

// SetPermissionRules 登记某插件的动态权限规则（覆盖式，可重复调用），由插件生命周期调用。
//
// 登记时会做两道防御性校验，不合规的规则丢弃并告警（而非让整个插件启动失败）：
//  1. 规则引用的权限点必须在插件自己声明的分组里——写错一个字不会在编译期暴露，
//     但会让该路由对所有非超管角色永久 403；
//  2. path 与 prefix 互斥、至少要有其一，否则规则永远无法命中。
func SetPermissionRules(name string, provider PermissionProvider) {
	if provider == nil {
		return
	}
	declared := make(map[string]bool)
	for _, g := range provider.PermissionGroups() {
		for _, p := range g.Permissions {
			declared[p] = true
		}
	}

	valid := make([]RoutePermRule, 0, len(provider.PermissionRules()))
	for _, r := range provider.PermissionRules() {
		if (r.Path == "") == (r.Prefix == "") {
			glog.Warningf(context.Background(),
				"plugin %s: 权限规则 %s %s 的 path 与 prefix 必须二选一，已丢弃", name, r.Method, r.Prefix+r.Path)
			continue
		}
		if !declared[r.Perm] {
			glog.Warningf(context.Background(),
				"plugin %s: 权限规则 %s %s 引用了未声明的权限点 %q，已丢弃（请在 PermissionGroups 中声明该权限点）",
				name, r.Method, r.Prefix+r.Path, r.Perm)
			continue
		}
		// 方法归一化为大写：插件作者写小写 "get" 不会在编译期暴露，
		// 但会让规则在匹配时永不命中，这里一次纠正
		r.Method = strings.ToUpper(strings.TrimSpace(r.Method))
		if r.Method == "" {
			glog.Warningf(context.Background(),
				"plugin %s: 权限规则 %s 缺少 HTTP 方法，已丢弃", name, r.Prefix+r.Path)
			continue
		}
		valid = append(valid, r)
	}

	permRulesMu.Lock()
	defer permRulesMu.Unlock()
	permRuleOwners[name] = valid
}

// ClearPermissionRules 摘除某插件的全部动态权限规则（禁用/卸载时调用）。
// 摘除后其路由回落到默认拒绝，权限点本身仍保持有效（见 RegisteredPermissionGroups）。
func ClearPermissionRules(name string) {
	permRulesMu.Lock()
	defer permRulesMu.Unlock()
	delete(permRuleOwners, name)
}

// DynamicPermissionRules 返回当前已登记的全部动态规则快照（middleware 匹配用）。
// 登记条目在插件启停时整体替换，不原地修改，快照读取无竞态。
func DynamicPermissionRules() []RoutePermRule {
	permRulesMu.RLock()
	defer permRulesMu.RUnlock()
	total := 0
	for _, rules := range permRuleOwners {
		total += len(rules)
	}
	if total == 0 {
		return nil
	}
	out := make([]RoutePermRule, 0, total)
	for _, rules := range permRuleOwners {
		out = append(out, rules...)
	}
	return out
}

// RegisteredPermissionGroups 收集【全部已注册插件】（不论安装状态）声明的权限点分组。
//
// 供权限点有效性校验（sanitizeGrantedPermissions）与超管全量权限集使用。
// 刻意不限安装/启用状态：插件是编译期注册的，其权限点随二进制存在，恒为「已定义」；
// 若按启用状态收缩，插件被禁用/卸载后，仍持有其权限点的角色会在下一次保存时
// 因「无效的权限点」而无法保存——权限点失效不应连锁破坏角色编辑。
// 插件未启用时其路由本来就被默认拒绝，保留权限点有效性没有放行风险。
func RegisteredPermissionGroups() []v1.PermissionGroup {
	return collectPermissionGroups(func(*PluginEntry) bool { return true })
}

// EnabledPermissionGroups / EnabledPermissionModules 收集【已启用】插件声明的
// 分组与模块档位元数据，供角色配置界面（GetAllPermissions）展示：
// 只呈现当前真正生效的插件模块，未启用插件的模块不占用授权界面。
func EnabledPermissionGroups() []v1.PermissionGroup {
	return collectPermissionGroups(func(e *PluginEntry) bool { return e.Status == StatusEnabled })
}

func EnabledPermissionModules() []v1.PermissionModuleMeta {
	var out []v1.PermissionModuleMeta
	for _, entry := range AllPlugins() {
		if entry.Status != StatusEnabled {
			continue
		}
		if pp, ok := entry.Plugin.(PermissionProvider); ok {
			out = append(out, pp.PermissionModules()...)
		}
	}
	return out
}

func collectPermissionGroups(match func(*PluginEntry) bool) []v1.PermissionGroup {
	var out []v1.PermissionGroup
	for _, entry := range AllPlugins() {
		if !match(entry) {
			continue
		}
		if pp, ok := entry.Plugin.(PermissionProvider); ok {
			out = append(out, pp.PermissionGroups()...)
		}
	}
	return out
}
