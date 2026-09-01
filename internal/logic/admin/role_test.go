package admin

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────
// 档位模型
// ─────────────────────────────────────────────────────────────────────────────

// TestTierIsSuperset 高档位必须包含低档位的全部权限点。
// 档位是「递进」语义，若高档漏了低档的某个权限，用户从「操作」升到「完全」反而会丢权限。
func TestTierIsSuperset(t *testing.T) {
	for module, defs := range tierPermissions {
		read, hasRead := defs[TierRead]
		operate, hasOperate := defs[TierOperate]
		full, hasFull := defs[TierFull]

		if hasRead && hasOperate {
			assertSubset(t, module, TierRead, read, TierOperate, operate)
		}
		if hasOperate && hasFull {
			assertSubset(t, module, TierOperate, operate, TierFull, full)
		}
		if hasRead && hasFull {
			assertSubset(t, module, TierRead, read, TierFull, full)
		}
	}
}

func assertSubset(t *testing.T, module, loTier string, lo []string, hiTier string, hi []string) {
	t.Helper()
	set := make(map[string]bool, len(hi))
	for _, p := range hi {
		set[p] = true
	}
	for _, p := range lo {
		if !set[p] {
			t.Errorf("模块 %s：%s 档的 %q 未包含在 %s 档中，升档会丢权限", module, loTier, p, hiTier)
		}
	}
}

// TestTierPermissionsAreDefined 档位映射表里的权限点必须真实存在。
// 写错一个字不会有编译期报错，但会让该档位永久失效。
func TestTierPermissionsAreDefined(t *testing.T) {
	valid := buildValidPermissionSet()
	for module, defs := range tierPermissions {
		for tier, perms := range defs {
			for _, p := range perms {
				if !valid[p] {
					t.Errorf("模块 %s 的 %s 档引用了未定义的权限点 %q", module, tier, p)
				}
				if !strings.HasPrefix(p, module+":") {
					t.Errorf("模块 %s 的 %s 档混入了其他模块的权限点 %q", module, tier, p)
				}
			}
		}
	}
}

// TestEveryPermissionGroupHasTiers 每个权限点分组都要有对应的档位定义，
// 否则该模块在角色配置界面上无法配置（只能走高级模式），等于遗漏。
func TestEveryPermissionGroupHasTiers(t *testing.T) {
	for _, grp := range predefinedPermissionGroups {
		if _, ok := tierPermissions[grp.Name]; !ok {
			t.Errorf("权限分组 %q 没有档位定义，角色界面将无法配置该模块", grp.Name)
		}
	}
}

// TestModuleTiersFoldsEquivalent 等价档位必须折叠，界面不渲染无意义的选项。
func TestModuleTiersFoldsEquivalent(t *testing.T) {
	cases := map[string][]string{
		"dashboard": {TierNone, TierRead},                        // 只有 view，无操作/完全之分
		"promo":     {TierNone, TierRead, TierOperate},           // 没有删除动作，无「完全」档
		"order":     {TierNone, TierRead, TierFull},              // 只读 → 含退款，没有中间档
		"tenant":    {TierNone, TierRead, TierOperate, TierFull}, // 三档齐全
	}
	for module, want := range cases {
		got := ModuleTiers(module)
		if !equalStrings(got, want) {
			t.Errorf("ModuleTiers(%q) = %v, want %v", module, got, want)
		}
	}
}

// TestTierRoundTrip 档位 → 权限点 → 档位 必须能原样还原，
// 否则界面在「保存后重新打开」时会把用户选的档位显示成别的档。
func TestTierRoundTrip(t *testing.T) {
	for module := range tierPermissions {
		for _, tier := range ModuleTiers(module) {
			perms := TierPermissions(module, tier)
			if got := TierForPermissions(module, perms); got != tier {
				t.Errorf("模块 %s：档位 %s 的权限集反推为 %q，未能还原", module, tier, got)
			}
		}
	}
}

// TestTierForPermissionsCustom 不匹配任何档位的权限组合应返回空串（界面显示「自定义」）。
func TestTierForPermissionsCustom(t *testing.T) {
	// 只给 delete 不给 view：不属于任何档位
	if got := TierForPermissions("model", []string{"model:delete"}); got != "" {
		t.Errorf("自定义权限组合应返回空串，实际 %q", got)
	}
	// 无该模块任何权限 = none
	if got := TierForPermissions("model", []string{"tenant:view"}); got != TierNone {
		t.Errorf("无该模块权限应返回 none，实际 %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 预置角色：锁住安全关键的取舍
// ─────────────────────────────────────────────────────────────────────────────

// TestBuiltinRoleDefaultsAreValid 预置角色的权限点必须真实存在且无重复。
func TestBuiltinRoleDefaultsAreValid(t *testing.T) {
	valid := buildValidPermissionSet()
	for code, perms := range builtinRoleDefaults {
		seen := make(map[string]bool, len(perms))
		for _, p := range perms {
			if !valid[p] {
				t.Errorf("预置角色 %s 引用了未定义的权限点 %q", code, p)
			}
			if seen[p] {
				t.Errorf("预置角色 %s 重复声明权限点 %q", code, p)
			}
			seen[p] = true
		}
	}
}

// TestBuiltinRoleBoundaries 锁定三个角色的分界线。
//
// 这些不是实现细节而是安全决策：改动它们意味着改变谁能碰钱、谁能改系统、谁能提权。
// 本测试让这类改动必须是显式的。
func TestBuiltinRoleBoundaries(t *testing.T) {
	has := func(code, perm string) bool {
		for _, p := range builtinRoleDefaults[code] {
			if p == perm {
				return true
			}
		}
		return false
	}

	// 地基：能改权限的人只能是超级管理员。
	// 任何预置角色拿到 user:* 都意味着它能给自己加满权限，角色体系随即失效。
	for code := range builtinRoleDefaults {
		for _, perm := range []string{"user:view", "user:create", "user:edit", "user:delete"} {
			if has(code, perm) {
				t.Errorf("预置角色 %s 不应拥有账号管理权限 %s —— 能改权限的人只能是超级管理员", code, perm)
			}
		}
	}

	// 插件是代码执行面、更新是版本变更，仅限超管
	for code := range builtinRoleDefaults {
		for _, perm := range []string{"system:plugin", "system:update", "audit:read_sensitive"} {
			if has(code, perm) {
				t.Errorf("预置角色 %s 不应拥有 %s（仅限超级管理员）", code, perm)
			}
		}
	}

	// 管理员 = 能碰钱与系统配置；这正是它与运营分开存在的理由
	for _, perm := range []string{"billing:refund", "order:refund", "system:edit"} {
		if !has("admin", perm) {
			t.Errorf("管理员应拥有 %s（去掉退款与系统设置，管理员与运营就没有分开的必要）", perm)
		}
	}

	// 运营 = 业务配置全权，但碰不了钱
	for _, perm := range []string{"channel:edit", "channel:delete", "model:edit", "plan:edit"} {
		if !has("operator", perm) {
			t.Errorf("运营应拥有 %s（平台运营的本职工作就是配渠道配模型）", perm)
		}
	}
	for _, perm := range []string{"billing:refund", "order:refund", "system:edit", "system:view", "audit:view"} {
		if has("operator", perm) {
			t.Errorf("运营不应拥有 %s", perm)
		}
	}
	// 租户停在「操作」档：能停服（可撤销），不能删除与销户（不可逆）
	if !has("operator", "tenant:suspend") {
		t.Error("运营应拥有 tenant:suspend（欠费/违规停服是日常运营手段）")
	}
	for _, perm := range []string{"tenant:delete", "tenant:close"} {
		if has("operator", perm) {
			t.Errorf("运营不应拥有 %s（不可逆操作留给管理员）", perm)
		}
	}

	// 技术支持 = 除财务面外全站只读，唯一写权限是工单
	for _, perm := range []string{"support:reply", "support:edit", "audit:view", "monitor:view", "channel:view"} {
		if !has("support", perm) {
			t.Errorf("技术支持应拥有 %s（排查问题所需）", perm)
		}
	}
	for _, perm := range []string{"billing:view", "order:view", "promo:view", "redemption:view", "invoice:view"} {
		if has("support", perm) {
			t.Errorf("技术支持不应看到财务数据 %s", perm)
		}
	}
	// 除 support:* 外不得有任何写权限
	for _, p := range builtinRoleDefaults["support"] {
		if strings.HasPrefix(p, "support:") {
			continue
		}
		if !strings.HasSuffix(p, ":view") {
			t.Errorf("技术支持除工单外应全部只读，但拥有写权限 %s", p)
		}
	}
}

// TestBuiltinRoleDefaultsMatchMigration 保证 Go 侧的「恢复默认权限」与迁移种子一致。
//
// 两处各存一份权限清单是必要的（迁移是 SQL、恢复默认是 Go），但漂移后果隐蔽：
// 新装部署拿到迁移的版本，点了「恢复默认」却变成 Go 的版本，同一个角色在不同部署里权限不同。
func TestBuiltinRoleDefaultsMatchMigration(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/000020_admin_role_management.sql")
	if err != nil {
		t.Fatalf("读取迁移脚本失败: %v", err)
	}
	sql := string(raw)

	// 匹配 unnest(ARRAY[ ... ]) ... WHERE r.code = 'xxx'
	blockRe := regexp.MustCompile(`(?s)unnest\(ARRAY\[(.*?)\]\).*?r\.code = '([a-z_]+)'`)
	permRe := regexp.MustCompile(`'([a-z_]+:[a-z_]+)'`)

	found := make(map[string][]string)
	for _, m := range blockRe.FindAllStringSubmatch(sql, -1) {
		body, code := m[1], m[2]
		var perms []string
		for _, pm := range permRe.FindAllStringSubmatch(body, -1) {
			perms = append(perms, pm[1])
		}
		found[code] = perms
	}

	if len(found) != len(builtinRoleDefaults) {
		t.Fatalf("迁移脚本里解析到 %d 个角色种子，Go 侧定义了 %d 个",
			len(found), len(builtinRoleDefaults))
	}

	for code, want := range builtinRoleDefaults {
		got, ok := found[code]
		if !ok {
			t.Errorf("迁移脚本缺少角色 %s 的权限种子", code)
			continue
		}
		if !equalStringSets(got, want) {
			t.Errorf("角色 %s 的权限在迁移脚本与 Go 定义之间不一致：\n  仅迁移有: %v\n  仅 Go 有: %v",
				code, difference(got, want), difference(want, got))
		}
	}
}

// TestDangerousPermissionsAreDefined 高危权限清单必须引用真实存在的权限点。
func TestDangerousPermissionsAreDefined(t *testing.T) {
	valid := buildValidPermissionSet()
	for p := range dangerousPermissions {
		if !valid[p] {
			t.Errorf("高危权限清单引用了未定义的权限点 %q", p)
		}
	}
	// 资金与提权类必须在清单内 —— 它们是二次确认的核心对象
	for _, p := range []string{"billing:refund", "order:refund", "user:edit", "system:edit"} {
		if _, ok := IsDangerousPermission(p); !ok {
			t.Errorf("%s 应被标记为高危权限", p)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 角色标识校验
// ─────────────────────────────────────────────────────────────────────────────

func TestRoleCodeValidation(t *testing.T) {
	valid := []string{"operator", "sales", "ops_l2", "support2", "ab"}
	invalid := []string{
		"",                      // 空
		"a",                     // 太短
		"Operator",              // 大写
		"2ops",                  // 数字开头
		"ops-l2",                // 连字符
		"ops l2",                // 空格
		"客服",                    // 中文
		strings.Repeat("a", 51), // 超长
	}
	for _, c := range valid {
		if !roleCodeRegexp.MatchString(c) {
			t.Errorf("角色标识 %q 应被接受", c)
		}
	}
	for _, c := range invalid {
		if roleCodeRegexp.MatchString(c) {
			t.Errorf("角色标识 %q 应被拒绝", c)
		}
	}
	// super_admin 形式上合法，但属于保留字，由业务层单独拦截
	if !roleCodeRegexp.MatchString(reservedRoleCode) {
		t.Error("保留字 super_admin 形式上应合法（由业务层按保留字拦截，而非靠正则）")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 辅助
// ─────────────────────────────────────────────────────────────────────────────

func TestSamePermissionSet(t *testing.T) {
	if !samePermissionSet([]string{"a", "b"}, []string{"b", "a"}) {
		t.Error("顺序不同的相同集合应判定为相等")
	}
	if samePermissionSet([]string{"a"}, []string{"a", "b"}) {
		t.Error("长度不同的集合应判定为不等")
	}
	if samePermissionSet(nil, []string{"a"}) {
		t.Error("nil 与非空集合应判定为不等")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringSets(a, b []string) bool {
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	return equalStrings(sa, sb)
}

// difference 返回在 a 中但不在 b 中的元素。
func difference(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}
