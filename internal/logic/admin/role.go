package admin

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"
)

// ─────────────────────────────────────────────────────────────────────────────
// 权限档位模型
// ─────────────────────────────────────────────────────────────────────────────

// 权限档位：配置界面按「模块 × 档位」呈现，而不是把 60 个权限点摊成复选框。
// 档位按【可逆性】而非【读/写/删】划分 —— 典型例子是租户：停用（suspend）是可撤销的
// 日常运营动作，删除与销户不可逆。若把三者塞进同一档，就会被迫在「运营不能停服」和
// 「运营能删客户」之间二选一。
const (
	TierNone    = "none"    // 无：该模块不可见
	TierRead    = "read"    // 只读：只能查看
	TierOperate = "operate" // 操作：日常增改与业务动作
	TierFull    = "full"    // 完全：含删除与高风险动作
)

// tierPermissions 是「模块 + 档位 → 权限点集合」的静态映射表。
//
// 刻意手写而非按 view/create/edit/delete 规则化推导：各模块的权限点语义差异太大
// （billing:refund 是资金操作、audit:read_sensitive 是隐私、member:model_scope 是配置），
// 规则化只会产生错误的归类。高档位包含低档位的全部权限点。
var tierPermissions = map[string]map[string][]string{
	"dashboard": {
		TierRead: {"dashboard:view"},
	},
	"tenant": {
		TierRead:    {"tenant:view"},
		TierOperate: {"tenant:view", "tenant:create", "tenant:edit", "tenant:suspend"},
		TierFull:    {"tenant:view", "tenant:create", "tenant:edit", "tenant:suspend", "tenant:delete", "tenant:close"},
	},
	"member": {
		TierRead:    {"member:view"},
		TierOperate: {"member:view", "member:import", "member:manage"},
		TierFull:    {"member:view", "member:import", "member:manage", "member:model_scope"},
	},
	"channel": {
		TierRead:    {"channel:view"},
		TierOperate: {"channel:view", "channel:create", "channel:edit", "channel:test"},
		TierFull:    {"channel:view", "channel:create", "channel:edit", "channel:test", "channel:delete"},
	},
	"model": {
		TierRead:    {"model:view"},
		TierOperate: {"model:view", "model:create", "model:edit"},
		TierFull:    {"model:view", "model:create", "model:edit", "model:delete"},
	},
	"billing": {
		TierRead:    {"billing:view"},
		TierOperate: {"billing:view", "billing:export"},
		TierFull:    {"billing:view", "billing:export", "billing:refund"},
	},
	"plan": {
		TierRead:    {"plan:view"},
		TierOperate: {"plan:view", "plan:create", "plan:edit"},
		TierFull:    {"plan:view", "plan:create", "plan:edit", "plan:delete"},
	},
	"order": {
		TierRead: {"order:view"},
		TierFull: {"order:view", "order:refund"},
	},
	"promo": {
		TierRead:    {"promo:view"},
		TierOperate: {"promo:view", "promo:create", "promo:edit"},
	},
	"redemption": {
		TierRead:    {"redemption:view"},
		TierOperate: {"redemption:view", "redemption:create", "redemption:edit"},
	},
	"invoice": {
		TierRead:    {"invoice:view"},
		TierOperate: {"invoice:view", "invoice:manage"},
	},
	"operation": {
		TierRead:    {"operation:view"},
		TierOperate: {"operation:view", "operation:edit"},
	},
	"support": {
		TierRead:    {"support:view"},
		TierOperate: {"support:view", "support:reply"},
		TierFull:    {"support:view", "support:reply", "support:edit"},
	},
	"monitor": {
		TierRead:    {"monitor:view"},
		TierOperate: {"monitor:view", "monitor:edit"},
	},
	"task": {
		TierRead:    {"task:view"},
		TierOperate: {"task:view", "task:edit"},
	},
	"file": {
		TierRead: {"file:view"},
		TierFull: {"file:view", "file:delete", "file:cleanup"},
	},
	"audit": {
		TierRead:    {"audit:view"},
		TierOperate: {"audit:view", "audit:export"},
		TierFull:    {"audit:view", "audit:export", "audit:read_sensitive", "audit:clear"},
	},
	"user": {
		TierRead:    {"user:view"},
		TierOperate: {"user:view", "user:create", "user:edit"},
		TierFull:    {"user:view", "user:create", "user:edit", "user:delete"},
	},
	"system": {
		TierRead:    {"system:view"},
		TierOperate: {"system:view", "system:edit"},
		// 在线自更新硬性限定超管、无权限点，「完全」档与「操作」档的差异只剩插件
		TierFull: {"system:view", "system:edit", "system:plugin"},
	},
}

// dangerousPermissions 是需要在界面上标红并二次确认的高危权限点。
// 四类：资金、隐私、系统、提权。
var dangerousPermissions = map[string]string{
	"billing:refund":       "可对任意租户钱包退款/调账",
	"order:refund":         "可对订单发起退款到支付渠道",
	"audit:read_sensitive": "可查看敏感数据访问日志",
	"audit:clear":          "可硬删除全部拦截日志（不可恢复）",
	"system:edit":          "可修改支付、汇率、存储、邮件等全站配置",
	"system:plugin":        "可安装/启停插件（等同于部署代码）",
	"user:create":          "可创建管理员账号",
	"user:edit":            "可修改管理员账号与其角色（含提权）",
	"user:delete":          "可删除管理员账号",
	"member:manage":        "可重置任意租户成员的密码（等同接管该成员账号）",
}

// tierOrder 定义档位由低到高的顺序，用于「权限点集合 → 档位」的反向推导。
var tierOrder = []string{TierNone, TierRead, TierOperate, TierFull}

// roleCodeRegexp 限制角色标识的取值：小写字母、数字、下划线，2-50 字符。
// code 参与权限缓存键与审计日志，必须稳定且可读。
var roleCodeRegexp = regexp.MustCompile(`^[a-z][a-z0-9_]{1,49}$`)

// reservedRoleCode 是保留字：超级管理员由 sys_admin_users.role 判定并短路鉴权，
// 不允许在角色表里出现同名角色造成语义混淆。
const reservedRoleCode = "super_admin"

// TierPermissions 返回指定模块与档位对应的权限点集合。
// 档位不存在（如 dashboard 没有 operate 档）时返回 nil，调用方应回落到更低档位。
func TierPermissions(module, tier string) []string {
	return tierPermissions[module][tier]
}

// ModuleTiers 返回某模块实际存在差异的档位列表（含 none，由低到高）。
//
// 界面只渲染这些档位：dashboard 只有 无/只读，promo 只有 无/只读/操作。
// 避免让用户在「操作」和「完全」两个完全等价的选项之间纠结。
func ModuleTiers(module string) []string {
	defs, ok := tierPermissions[module]
	if !ok {
		return []string{TierNone}
	}
	out := []string{TierNone}
	var prev []string
	for _, tier := range tierOrder[1:] {
		perms, exists := defs[tier]
		if !exists || samePermissionSet(perms, prev) {
			continue
		}
		out = append(out, tier)
		prev = perms
	}
	return out
}

// TierForPermissions 从一组权限点反推它属于某模块的哪个档位。
// 完全匹配某档位时返回该档位，否则返回空串表示「自定义」（界面切到高级模式展示）。
func TierForPermissions(module string, perms []string) string {
	owned := make([]string, 0, len(perms))
	prefix := module + ":"
	for _, p := range perms {
		if strings.HasPrefix(p, prefix) {
			owned = append(owned, p)
		}
	}
	if len(owned) == 0 {
		return TierNone
	}
	// 由高到低比对，命中即返回（高档位是低档位的超集，先比高档避免误判）
	for i := len(tierOrder) - 1; i >= 1; i-- {
		if defs, ok := tierPermissions[module][tierOrder[i]]; ok && samePermissionSet(owned, defs) {
			return tierOrder[i]
		}
	}
	return ""
}

// IsDangerousPermission 判断权限点是否属于需要二次确认的高危权限。
func IsDangerousPermission(perm string) (string, bool) {
	reason, ok := dangerousPermissions[perm]
	return reason, ok
}

func samePermissionSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := append([]string(nil), a...)
	sb := append([]string(nil), b...)
	sort.Strings(sa)
	sort.Strings(sb)
	for i := range sa {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}

// ─────────────────────────────────────────────────────────────────────────────
// 有效权限计算与缓存
// ─────────────────────────────────────────────────────────────────────────────

// adminPermCache 缓存每个管理员的有效权限集合。
//
// 鉴权在每个请求上都要跑，此前是一次 COUNT 查询。复用项目的双层缓存
// （L1 gcache + L2 Redis + 跨实例失效订阅），TTL 与「用户信息 300s」的既定策略一致。
var adminPermCache = lcommon.NewCache("admin_perm", 300*time.Second)

// permCacheKey 按用户维度组织缓存键。
func permCacheKey(userID int64) string {
	return g.NewVar(userID).String()
}

// InvalidateUserPermCache 清除单个用户的权限缓存（角色关联、特批权限变更时调用）。
func InvalidateUserPermCache(ctx context.Context, userID int64) {
	adminPermCache.Delete(ctx, permCacheKey(userID))
}

// InvalidateAllPermCache 清空全部管理员的权限缓存。
//
// 角色维度的变更（改角色权限、启停、删除）会影响该角色下的所有用户。管理员账号在
// 几十量级且这类操作低频，直接清空整个前缀比精确定位受影响用户更简单也更不容易漏。
func InvalidateAllPermCache(ctx context.Context) {
	adminPermCache.DeleteByPattern(ctx, "*")
}

// GetEffectivePermissions 返回管理员的有效权限点集合。
//
//	有效权限 = ∪(已启用角色的权限) ∪ 用户特批权限(sys_admin_role_perms)
//
// 只做并集不做否定：带 deny 规则的 RBAC 在排查「这个人为什么没权限」时需要回溯整条
// 规则链，运维成本远大于收益。要收权限就去掉角色或改角色。
//
// super_admin 不查表，直接返回全部权限点。
func GetEffectivePermissions(ctx context.Context, userID int64, role string) ([]string, error) {
	if role == "super_admin" {
		return AllPermissionPoints(), nil
	}

	var cached []string
	if adminPermCache.GetJSON(ctx, permCacheKey(userID), &cached) {
		return cached, nil
	}

	perms, err := loadEffectivePermissions(ctx, userID)
	if err != nil {
		return nil, err
	}
	adminPermCache.Set(ctx, permCacheKey(userID), perms)
	return perms, nil
}

// loadEffectivePermissions 从数据库计算有效权限（不走缓存）。
func loadEffectivePermissions(ctx context.Context, userID int64) ([]string, error) {
	set := make(map[string]bool)

	// 1) 角色权限：仅统计已启用的角色。
	// 角色被禁用后其权限立即失效，不需要解除用户关联。
	var rolePerms []struct {
		PermissionPoint string `json:"permission_point"`
	}
	err := dao.SysAdminRolePermissions.Ctx(ctx).
		As("rp").
		LeftJoin("sys_admin_roles r", "r.id = rp.role_id").
		LeftJoin("sys_admin_user_roles ur", "ur.role_id = rp.role_id").
		Where("ur.admin_user_id", userID).
		Where("r.is_enabled", true).
		Fields("rp.permission_point").
		Scan(&rolePerms)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	for _, p := range rolePerms {
		set[p.PermissionPoint] = true
	}

	// 2) 用户特批权限：独立于角色，角色禁用不影响特批。
	var extraPerms []struct {
		PermissionPoint string `json:"permission_point"`
	}
	err = dao.SysAdminRolePerms.Ctx(ctx).
		Where("admin_user_id", userID).
		Fields("permission_point").
		Scan(&extraPerms)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	for _, p := range extraPerms {
		set[p.PermissionPoint] = true
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

// AllPermissionPoints 返回系统定义的全部权限点（超级管理员使用）。
func AllPermissionPoints() []string {
	var out []string
	for _, grp := range predefinedPermissionGroups {
		out = append(out, grp.Permissions...)
	}
	sort.Strings(out)
	return out
}

// ─────────────────────────────────────────────────────────────────────────────
// 用户角色关联
// ─────────────────────────────────────────────────────────────────────────────

// GetUserRoleBriefs 返回用户已分配角色的简要信息（登录响应与用户列表展示用）。
func GetUserRoleBriefs(ctx context.Context, userID int64) ([]v1.AdminRoleBrief, error) {
	var rows []v1.AdminRoleBrief
	err := dao.SysAdminUserRoles.Ctx(ctx).
		As("ur").
		LeftJoin("sys_admin_roles r", "r.id = ur.role_id").
		Where("ur.admin_user_id", userID).
		Fields("r.id, r.code, r.name, r.is_enabled").
		OrderAsc("r.sort").
		Scan(&rows)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	return rows, nil
}

// replaceUserRoles 全量覆盖用户的角色关联（调用方负责事务与缓存失效）。
func replaceUserRoles(ctx context.Context, userID int64, roleIDs []int64) error {
	if _, err := dao.SysAdminUserRoles.Ctx(ctx).
		Where("admin_user_id", userID).Delete(); err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}

	// 去重，避免前端重复提交同一角色触发唯一约束
	seen := make(map[int64]bool, len(roleIDs))
	data := make([]do.SysAdminUserRoles, 0, len(roleIDs))
	for _, id := range roleIDs {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		data = append(data, do.SysAdminUserRoles{AdminUserId: userID, RoleId: id})
	}
	if len(data) == 0 {
		return nil
	}

	// 校验角色存在，避免写入悬空关联（无外键，DB 不会拦）
	count, err := dao.SysAdminRoles.Ctx(ctx).WhereIn("id", roleIDs).Count()
	if err != nil {
		return err
	}
	if count != len(data) {
		return lcommon.NewBadRequestError("包含不存在的角色")
	}

	_, err = dao.SysAdminUserRoles.Ctx(ctx).Data(data).Insert()
	return err
}

// AssignUserRoles 为管理员分配角色（全量覆盖）。
func (s *sAdmin) AssignUserRoles(ctx context.Context, req *v1.AdminUserRoleAssignReq) (*v1.AdminUserRoleAssignRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	user, err := loadAdminUserRole(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if user.Role == "super_admin" {
		return nil, lcommon.NewBadRequestError("超级管理员拥有全部权限，无需分配角色")
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return replaceUserRoles(ctx, req.Id, req.RoleIDs)
	})
	if err != nil {
		return nil, err
	}

	InvalidateUserPermCache(ctx, req.Id)
	return &v1.AdminUserRoleAssignRes{}, nil
}

// GetUserRoles 查询管理员已分配的角色。
func (s *sAdmin) GetUserRoles(ctx context.Context, req *v1.AdminUserRoleListReq) (*v1.AdminUserRoleListRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := loadAdminUserRole(ctx, req.Id); err != nil {
		return nil, err
	}
	briefs, err := GetUserRoleBriefs(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	if briefs == nil {
		briefs = []v1.AdminRoleBrief{}
	}
	return &v1.AdminUserRoleListRes{List: briefs}, nil
}

// loadAdminUserRole 读取管理员账号的特权标记，顺带校验账号存在。
func loadAdminUserRole(ctx context.Context, userID int64) (*struct {
	Role string `json:"role"`
}, error) {
	var user *struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Fields("role").Scan(&user)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, lcommon.NewNotFoundError("用户")
	}
	return user, nil
}

// loadRolesByUsers 批量查询多个管理员的角色，返回 userID → 角色列表。
//
// 用户列表页要展示每个账号的角色，逐个查询会产生 N+1；这里一次 IN 查询后在内存里分组。
// 结果保证每个入参 userID 都有对应条目（无角色者为空切片），前端可直接据此提示「未分配角色」。
func loadRolesByUsers(ctx context.Context, userIDs []int64) (map[int64][]v1.AdminRoleBrief, error) {
	out := make(map[int64][]v1.AdminRoleBrief, len(userIDs))
	for _, id := range userIDs {
		out[id] = []v1.AdminRoleBrief{}
	}
	if len(userIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		AdminUserId int64  `json:"admin_user_id"`
		ID          int64  `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		IsEnabled   bool   `json:"is_enabled"`
	}
	err := dao.SysAdminUserRoles.Ctx(ctx).
		As("ur").
		LeftJoin("sys_admin_roles r", "r.id = ur.role_id").
		WhereIn("ur.admin_user_id", userIDs).
		Fields("ur.admin_user_id, r.id, r.code, r.name, r.is_enabled").
		OrderAsc("r.sort").
		Scan(&rows)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	for _, r := range rows {
		out[r.AdminUserId] = append(out[r.AdminUserId], v1.AdminRoleBrief{
			ID:        r.ID,
			Code:      r.Code,
			Name:      r.Name,
			IsEnabled: r.IsEnabled,
		})
	}
	return out, nil
}
