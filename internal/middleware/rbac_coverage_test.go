package middleware

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/qianfree/team-api/internal/logic/admin"
)

// adminAPIDir 是管理后台 API 定义所在目录（相对本测试文件）。
// 路由由 cmd.go 中 g.Bind(adminController.NewV1()) 从这些 g.Meta 注解注册，
// 前缀统一为 /api/admin。
const adminAPIDir = "../../api/admin/v1"

// adminRoutePrefix 与 internal/cmd/cmd.go 的 group.Group("/admin", ...) 保持一致。
const adminRoutePrefix = "/api/admin"

// manualAdminRoutes 列出不经 g.Bind(adminController) 注册、但同样挂在
// AdminPermissionGuard 之下的路由（见 internal/cmd/cmd.go 的独立路由组）。
// 这些路由没有 g.Meta 注解，AST 扫不到，需手工登记。
var manualAdminRoutes = []adminRoute{
	{Method: "GET", Path: "/api/admin/files/{id}/serve"},
}

type adminRoute struct {
	Method string
	Path   string
	Source string // 来源文件:行，用于报错定位
}

// collectAdminRoutes 解析 api/admin/v1 下全部 g.Meta 注解，得到已注册的 admin 路由清单。
//
// 这里刻意用 AST 解析源码而不是 ghttp.Server.GetRoutes()：后者需要导入
// internal/controller/admin，而它反向依赖 internal/middleware，会造成 import cycle。
// 解析源码同样精确（路由本就由这些注解生成），且不需要构造 server 或任何外部依赖。
func collectAdminRoutes(t *testing.T) []adminRoute {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(adminAPIDir, "*.go"))
	if err != nil {
		t.Fatalf("扫描 %s 失败: %v", adminAPIDir, err)
	}

	fset := token.NewFileSet()
	seen := make(map[string]bool)
	var out []adminRoute

	add := func(r adminRoute) {
		key := r.Method + " " + r.Path
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, r)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatalf("解析 %s 失败: %v", file, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}
			for _, field := range st.Fields.List {
				// g.Meta 是嵌入字段（无字段名），带 path/method 标签
				if len(field.Names) != 0 || field.Tag == nil {
					continue
				}
				raw, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					continue
				}
				tag := reflect.StructTag(raw)
				path := strings.TrimSpace(tag.Get("path"))
				if path == "" {
					continue
				}
				pos := fset.Position(field.Pos())
				for _, m := range strings.Split(tag.Get("method"), ",") {
					m = strings.ToUpper(strings.TrimSpace(m))
					if m == "" {
						m = "GET" // g.Meta 省略 method 时 GoFrame 默认注册 GET
					}
					add(adminRoute{
						Method: m,
						Path:   adminRoutePrefix + path,
						Source: filepath.Base(pos.Filename) + ":" + strconv.Itoa(pos.Line),
					})
				}
			}
			return true
		})
	}

	for _, r := range manualAdminRoutes {
		r.Source = "internal/cmd/cmd.go（手工注册）"
		add(r)
	}

	if len(out) == 0 {
		t.Fatalf("未从 %s 解析到任何路由，测试装置失效", adminAPIDir)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

// concreteFromPattern 把路由模式里的占位段替换成具体值，
// 使其可以喂给 matchPermission（它按具体路径做前缀/后缀匹配，不认识占位符）。
//
//	/api/admin/users/{id}/permissions -> /api/admin/users/1/permissions
//	/api/admin/channels/:id           -> /api/admin/channels/1
func concreteFromPattern(pattern string) string {
	segs := strings.Split(pattern, "/")
	for i, seg := range segs {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") || strings.HasPrefix(seg, ":") || strings.HasPrefix(seg, "*") {
			segs[i] = "1"
		}
	}
	return strings.Join(segs, "/")
}

// allPermissionPoints 收集 predefinedPermissionGroups 里声明的全部权限点，
// 外加中间件自身消费的伪权限点。
func allPermissionPoints() map[string]bool {
	set := map[string]bool{
		// self:access 不属于任何模块分组：它表示「登录即可访问自己的资源」，
		// 在 AdminPermissionGuard 里被直接放行，不参与角色授权。
		"self:access": true,
	}
	for _, g := range admin.PermissionGroups() {
		for _, p := range g.Permissions {
			set[p] = true
		}
	}
	return set
}

// TestEveryAdminRouteHasPermissionRule 保证每个受保护的 admin 接口都能匹配到权限规则。
//
// AdminPermissionGuard 对未匹配到规则的路由返回 403（默认拒绝）。若新增接口时忘了在
// adminPermissionRules 登记，低权限角色会拿到一个无法解释的 403 —— 本测试把这类遗漏
// 在 CI 阶段拦下，而不是等到线上有人点开某个页面才发现。
func TestEveryAdminRouteHasPermissionRule(t *testing.T) {
	var missing []string

	for _, r := range collectAdminRoutes(t) {
		path := concreteFromPattern(r.Path)
		if isAdminPublicPath(path) {
			continue
		}
		// 超管专属路由（角色管理、在线自更新）由 superAdminOnlyRules 硬性拦截，
		// 先于权限规则判定，不走权限点授权，因此不要求出现在 adminPermissionRules 里。
		if isSuperAdminOnly(r.Method, path) {
			continue
		}
		if perm := matchPermission(r.Method, path); perm == "" {
			missing = append(missing, r.Method+" "+r.Path+"  ("+r.Source+")")
		}
	}

	if len(missing) > 0 {
		t.Fatalf("以下 %d 个 admin 接口未配置权限规则（请在 adminPermissionRules 中补齐）：\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// TestPermissionRulesReferenceKnownPoints 保证规则里引用的权限点都是真实存在的。
//
// 规则里写错一个字（如 channel:veiw）不会有任何编译期报错，但会造成该接口对所有
// 非超管角色永久 403 —— 因为没有任何角色能被授予一个不存在的权限点。
func TestPermissionRulesReferenceKnownPoints(t *testing.T) {
	known := allPermissionPoints()

	for _, rule := range adminPermissionRules {
		if !known[rule.perm] {
			target := rule.path
			if target == "" {
				target = rule.prefix + "*" + rule.suffix
			}
			t.Errorf("规则 %s %s 引用了未定义的权限点 %q", rule.method, target, rule.perm)
		}
	}
}

// TestMatchedPermissionsAreGrantable 从路由侧再验一次：
// 每个受保护接口最终要求的权限点，都必须是可以授予某个角色的。
func TestMatchedPermissionsAreGrantable(t *testing.T) {
	known := allPermissionPoints()

	for _, r := range collectAdminRoutes(t) {
		path := concreteFromPattern(r.Path)
		if isAdminPublicPath(path) {
			continue
		}
		perm := matchPermission(r.Method, path)
		if perm == "" {
			continue // 由 TestEveryAdminRouteHasPermissionRule 负责报错
		}
		if !known[perm] {
			t.Errorf("接口 %s %s 要求权限点 %q，但该权限点未在 predefinedPermissionGroups 中定义",
				r.Method, r.Path, perm)
		}
	}
}

// TestRoleManagementIsSuperAdminOnly 保证角色管理的每个端点都被限定为超级管理员专属。
//
// 角色管理是权限体系的元操作：谁能改角色，谁就能决定所有人的权限。若某个角色端点漏了这条
// 限制，一个被下放了 user:edit 的账号就能给自己挂上更高权限的角色，整套角色体系当场失效。
// 新增角色相关路由时，本测试会强制你同步登记 superAdminOnlyRules。
func TestRoleManagementIsSuperAdminOnly(t *testing.T) {
	var leaked []string

	for _, r := range collectAdminRoutes(t) {
		path := concreteFromPattern(r.Path)
		isRoleRoute := strings.HasPrefix(r.Path, "/api/admin/roles") ||
			strings.HasSuffix(r.Path, "/roles")
		if !isRoleRoute {
			continue
		}
		if !isSuperAdminOnly(r.Method, path) {
			leaked = append(leaked, r.Method+" "+r.Path+"  ("+r.Source+")")
		}
	}

	if len(leaked) > 0 {
		t.Fatalf("以下角色管理接口未限定为超级管理员专属（请补入 superAdminOnlyRules）：\n  %s",
			strings.Join(leaked, "\n  "))
	}
}

// TestSuperAdminOnlyRulesDoNotOverreach 反向确认：超管专属清单不应误伤普通账号管理。
//
// 账号管理（创建/禁用/改密/删除）按 user:* 授权、可以正常下放，只有「分配角色」才是
// 权限体系的元操作。两者路径相邻（都在 /api/admin/users/ 下），容易被前缀规则误伤。
func TestSuperAdminOnlyRulesDoNotOverreach(t *testing.T) {
	shouldStayDelegatable := []struct{ method, path string }{
		{"GET", "/api/admin/users"},
		{"POST", "/api/admin/users"},
		{"PUT", "/api/admin/users/1"},
		{"PUT", "/api/admin/users/1/status"},
		{"PUT", "/api/admin/users/1/reset-password"},
		{"PUT", "/api/admin/users/1/unlock"},
		{"DELETE", "/api/admin/users/1"},
		{"GET", "/api/admin/users/1/permissions"},
	}
	for _, c := range shouldStayDelegatable {
		if isSuperAdminOnly(c.method, c.path) {
			t.Errorf("%s %s 属于普通账号管理，不应被限定为超管专属", c.method, c.path)
		}
	}
}
