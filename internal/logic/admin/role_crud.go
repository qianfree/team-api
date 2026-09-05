package admin

import (
	"context"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/do"
)

// builtinRoleDefaults 是三个预置角色的默认权限，用于「恢复默认权限」。
//
// 与 migrations/000020_admin_role_management.sql 的种子数据必须一致，
// 由 TestBuiltinRoleDefaultsMatchMigration 在 CI 阶段比对，防止两处漂移。
//
// 注意这些只是初始值而非系统契约：用户可以任意修改，甚至删掉预置角色自建一套。
var builtinRoleDefaults = map[string][]string{
	// 管理员：除 user:*（账号与角色）与 system:plugin 外的全部权限
	//（在线自更新为超管专属，不设权限点）。
	// 与运营的区别就是「能碰钱（退款）和系统配置」—— 去掉这两项，两个角色就没有分开的必要。
	"admin": {
		"dashboard:view",
		"channel:view", "channel:create", "channel:edit", "channel:test", "channel:delete",
		"model:view", "model:create", "model:edit", "model:delete",
		"tenant:view", "tenant:create", "tenant:edit", "tenant:suspend", "tenant:delete", "tenant:close",
		"member:view", "member:import", "member:manage", "member:model_scope",
		"plan:view", "plan:create", "plan:edit", "plan:delete",
		"billing:view", "billing:export", "billing:refund",
		"order:view", "order:refund",
		"promo:view", "promo:create", "promo:edit",
		"redemption:view", "redemption:create", "redemption:edit",
		"invoice:view", "invoice:manage",
		"operation:view", "operation:edit",
		"support:view", "support:reply", "support:edit",
		"monitor:view", "monitor:edit",
		"task:view", "task:edit",
		"file:view", "file:delete", "file:cleanup",
		"audit:view", "audit:export",
		"system:view", "system:edit",
	},
	// 运营：平台日常运营，渠道与模型的完整权限（配渠道就得填 Key，这是本职工作）。
	// 不给退款、审计、系统设置、账号管理。租户停在「操作」档：能停服，不能删除与销户。
	"operator": {
		"dashboard:view",
		"channel:view", "channel:create", "channel:edit", "channel:test", "channel:delete",
		"model:view", "model:create", "model:edit", "model:delete",
		"tenant:view", "tenant:create", "tenant:edit", "tenant:suspend",
		"member:view", "member:import", "member:manage", "member:model_scope",
		"plan:view", "plan:create", "plan:edit", "plan:delete",
		"billing:view",
		"order:view",
		"promo:view", "promo:create", "promo:edit",
		"redemption:view", "redemption:create", "redemption:edit",
		"invoice:view", "invoice:manage",
		"operation:view", "operation:edit",
		"support:view", "support:reply", "support:edit",
		"monitor:view", "monitor:edit",
		"task:view", "task:edit",
		"file:view", "file:delete", "file:cleanup",
	},
	// 技术支持：除财务面外全站只读 + 工单完整权限。
	// 排查主力工具不在财务面：请求审计日志走 audit:view，错误日志与渠道错误监控走 monitor:view。
	"support": {
		"dashboard:view",
		"channel:view",
		"model:view",
		"tenant:view",
		"member:view",
		"plan:view",
		"operation:view",
		"support:view", "support:reply", "support:edit",
		"monitor:view",
		"task:view",
		"file:view",
		"audit:view",
	},
}

// assertSuperAdmin 校验调用者是超级管理员。
//
// 与 middleware 的 superAdminOnlyRules 是两道独立的闸门：中间件按路由拦截，这里按方法拦截。
// 双重校验不是冗余 —— 路由表漏配、内部调用绕过 HTTP 层、将来有人给角色接口加了新路径，
// 任何一种情况下都还有一道守住。权限体系的元操作值得这个代价。
func assertSuperAdmin(ctx context.Context) error {
	if lcommon.GetCtxUserRole(ctx) == "super_admin" {
		return nil
	}
	return lcommon.NewBusinessError(consts.CodeForbidden, "角色权限管理仅超级管理员可用")
}

// ListRoles 返回角色列表（角色数量少，不分页）。
func (s *sAdmin) ListRoles(ctx context.Context, _ *v1.AdminRoleListReq) (*v1.AdminRoleListRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	var roles []struct {
		ID          int64  `json:"id"`
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		IsBuiltin   bool   `json:"is_builtin"`
		IsEnabled   bool   `json:"is_enabled"`
		Sort        int    `json:"sort"`
		CreatedAt   string `json:"created_at"`
	}
	err := dao.SysAdminRoles.Ctx(ctx).OrderAsc("sort").OrderAsc("id").Scan(&roles)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	userCounts, err := countByRole(ctx, dao.SysAdminUserRoles.Ctx(ctx))
	if err != nil {
		return nil, err
	}
	permCounts, err := countByRole(ctx, dao.SysAdminRolePermissions.Ctx(ctx))
	if err != nil {
		return nil, err
	}

	list := make([]v1.AdminRoleItem, len(roles))
	for i, r := range roles {
		list[i] = v1.AdminRoleItem{
			ID:          r.ID,
			Code:        r.Code,
			Name:        r.Name,
			Description: r.Description,
			IsBuiltin:   r.IsBuiltin,
			IsEnabled:   r.IsEnabled,
			Sort:        r.Sort,
			UserCount:   userCounts[r.ID],
			PermCount:   permCounts[r.ID],
			CreatedAt:   r.CreatedAt,
		}
	}
	return &v1.AdminRoleListRes{List: list}, nil
}

// countByRole 按 role_id 聚合计数，避免在角色列表里对每个角色各查一次（N+1）。
func countByRole(ctx context.Context, model *gdb.Model) (map[int64]int, error) {
	var rows []struct {
		RoleId int64 `json:"role_id"`
		Cnt    int   `json:"cnt"`
	}
	err := model.Fields("role_id, COUNT(*) AS cnt").Group("role_id").Scan(&rows)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(rows))
	for _, r := range rows {
		out[r.RoleId] = r.Cnt
	}
	return out, nil
}

// GetRoleDetail 返回角色详情，同时给出权限点全集与按模块归纳的档位视图。
func (s *sAdmin) GetRoleDetail(ctx context.Context, req *v1.AdminRoleDetailReq) (*v1.AdminRoleDetailRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	role, err := loadRole(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	perms, err := loadRolePermissions(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	userCount, err := dao.SysAdminUserRoles.Ctx(ctx).Where("role_id", req.ID).Count()
	if err != nil {
		return nil, err
	}

	return &v1.AdminRoleDetailRes{
		ID:          role.ID,
		Code:        role.Code,
		Name:        role.Name,
		Description: role.Description,
		IsBuiltin:   role.IsBuiltin,
		IsEnabled:   role.IsEnabled,
		Sort:        role.Sort,
		UserCount:   userCount,
		Permissions: perms,
		ModuleTiers: buildModuleTiers(perms),
		CreatedAt:   role.CreatedAt,
	}, nil
}

// buildModuleTiers 把权限点集合归纳成每个模块的档位选择，供界面按档位渲染。
func buildModuleTiers(perms []string) []v1.AdminRoleModuleTier {
	modules := make([]string, 0, len(tierPermissions))
	for m := range tierPermissions {
		modules = append(modules, m)
	}
	sort.Strings(modules)

	out := make([]v1.AdminRoleModuleTier, 0, len(modules))
	for _, m := range modules {
		out = append(out, v1.AdminRoleModuleTier{Module: m, Tier: TierForPermissions(m, perms)})
	}
	return out
}

// CreateRole 新建角色。权限可直接给出，也可从现有角色复制。
func (s *sAdmin) CreateRole(ctx context.Context, req *v1.AdminRoleCreateReq) (*v1.AdminRoleCreateRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	code := strings.ToLower(strings.TrimSpace(req.Code))
	if code == reservedRoleCode {
		return nil, lcommon.NewBadRequestError("super_admin 是保留标识，超级管理员由账号属性判定，不能作为角色")
	}
	if !roleCodeRegexp.MatchString(code) {
		return nil, lcommon.NewBadRequestError("角色标识需以小写字母开头，仅含小写字母、数字与下划线，长度 2-50")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, lcommon.NewBadRequestError("请输入角色名称")
	}

	exists, err := dao.SysAdminRoles.Ctx(ctx).Where("code", code).Count()
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		return nil, lcommon.NewBadRequestError("角色标识已存在：" + code)
	}

	// 权限来源：显式给出的权限点优先，其次从指定角色复制
	perms := req.Permissions
	if len(perms) == 0 && req.CopyFromRoleID > 0 {
		if _, err := loadRole(ctx, req.CopyFromRoleID); err != nil {
			return nil, err
		}
		if perms, err = loadRolePermissions(ctx, req.CopyFromRoleID); err != nil {
			return nil, err
		}
	}

	perms, err = sanitizeGrantedPermissions(ctx, perms)
	if err != nil {
		return nil, err
	}

	var newID int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		result, err := dao.SysAdminRoles.Ctx(ctx).Data(do.SysAdminRoles{
			Code:        code,
			Name:        name,
			Description: strings.TrimSpace(req.Description),
			IsBuiltin:   false,
			IsEnabled:   true,
			Sort:        req.Sort,
		}).Insert()
		if err != nil {
			if lcommon.IsDuplicateKeyError(err) {
				return lcommon.NewBadRequestError("角色标识已存在：" + code)
			}
			return err
		}
		newID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		return replaceRolePermissions(ctx, newID, perms)
	})
	if err != nil {
		return nil, err
	}

	return &v1.AdminRoleCreateRes{ID: newID}, nil
}

// UpdateRole 更新角色的名称、说明、排序与权限（code 不可修改）。
func (s *sAdmin) UpdateRole(ctx context.Context, req *v1.AdminRoleUpdateReq) (*v1.AdminRoleUpdateRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := loadRole(ctx, req.ID); err != nil {
		return nil, err
	}

	data := g.Map{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, lcommon.NewBadRequestError("角色名称不能为空")
		}
		data["name"] = name
	}
	if req.Description != nil {
		data["description"] = strings.TrimSpace(*req.Description)
	}
	if req.Sort != nil {
		data["sort"] = *req.Sort
	}

	var perms []string
	if req.Permissions != nil {
		var err error
		if perms, err = sanitizeGrantedPermissions(ctx, *req.Permissions); err != nil {
			return nil, err
		}
	}

	err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if len(data) > 0 {
			data["updated_at"] = gdb.Raw("now()")
			if _, err := dao.SysAdminRoles.Ctx(ctx).Where("id", req.ID).Data(data).Update(); err != nil {
				return err
			}
		}
		if req.Permissions != nil {
			return replaceRolePermissions(ctx, req.ID, perms)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 角色权限变更影响该角色下的全部用户，直接清空前缀
	if req.Permissions != nil {
		InvalidateAllPermCache(ctx)
	}
	return &v1.AdminRoleUpdateRes{}, nil
}

// UpdateRoleStatus 启用/禁用角色。禁用后该角色的权限对全部关联用户立即失效。
func (s *sAdmin) UpdateRoleStatus(ctx context.Context, req *v1.AdminRoleStatusUpdateReq) (*v1.AdminRoleStatusUpdateRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := loadRole(ctx, req.ID); err != nil {
		return nil, err
	}
	_, err := dao.SysAdminRoles.Ctx(ctx).Where("id", req.ID).Data(g.Map{
		"is_enabled": req.IsEnabled,
		"updated_at": gdb.Raw("now()"),
	}).Update()
	if err != nil {
		return nil, err
	}
	InvalidateAllPermCache(ctx)
	return &v1.AdminRoleStatusUpdateRes{}, nil
}

// DeleteRole 删除角色，并在同一事务内级联清理其权限行与用户关联。
//
// 无外键，级联必须由业务层完成，否则会留下悬空的权限行与用户关联。
// 预置角色同样可删：is_builtin 只标识来源，不构成删除保护。
//
// 不存在把自己锁死的风险 —— 超级管理员由 sys_admin_users.role 判定并短路鉴权，
// 不依赖任何角色记录，即便角色被删光也能登录并重建，因此不设「至少保留一个角色」的约束。
func (s *sAdmin) DeleteRole(ctx context.Context, req *v1.AdminRoleDeleteReq) (*v1.AdminRoleDeleteRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := loadRole(ctx, req.ID); err != nil {
		return nil, err
	}

	affected, err := dao.SysAdminUserRoles.Ctx(ctx).Where("role_id", req.ID).Count()
	if err != nil {
		return nil, err
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := dao.SysAdminUserRoles.Ctx(ctx).Where("role_id", req.ID).Delete(); err != nil {
			return err
		}
		if _, err := dao.SysAdminRolePermissions.Ctx(ctx).Where("role_id", req.ID).Delete(); err != nil {
			return err
		}
		_, err := dao.SysAdminRoles.Ctx(ctx).Where("id", req.ID).Delete()
		return err
	})
	if err != nil {
		return nil, err
	}

	InvalidateAllPermCache(ctx)
	return &v1.AdminRoleDeleteRes{AffectedUsers: affected}, nil
}

// ResetRoleDefaults 把预置角色的权限恢复为出厂默认值。
func (s *sAdmin) ResetRoleDefaults(ctx context.Context, req *v1.AdminRoleResetReq) (*v1.AdminRoleResetRes, error) {
	if err := assertSuperAdmin(ctx); err != nil {
		return nil, err
	}
	role, err := loadRole(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if !role.IsBuiltin {
		return nil, lcommon.NewBadRequestError("只有系统预置角色才有默认权限可恢复")
	}
	defaults, ok := builtinRoleDefaults[role.Code]
	if !ok {
		return nil, lcommon.NewBadRequestError("该角色没有登记默认权限")
	}

	perms, err := sanitizeGrantedPermissions(ctx, defaults)
	if err != nil {
		return nil, err
	}

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		return replaceRolePermissions(ctx, req.ID, perms)
	})
	if err != nil {
		return nil, err
	}

	InvalidateAllPermCache(ctx)
	return &v1.AdminRoleResetRes{Permissions: perms}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// 内部辅助
// ─────────────────────────────────────────────────────────────────────────────

type roleRecord struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IsBuiltin   bool   `json:"is_builtin"`
	IsEnabled   bool   `json:"is_enabled"`
	Sort        int    `json:"sort"`
	CreatedAt   string `json:"created_at"`
}

func loadRole(ctx context.Context, id int64) (*roleRecord, error) {
	var role *roleRecord
	err := dao.SysAdminRoles.Ctx(ctx).Where("id", id).Scan(&role)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if role == nil {
		return nil, lcommon.NewNotFoundError("角色")
	}
	return role, nil
}

func loadRolePermissions(ctx context.Context, roleID int64) ([]string, error) {
	var rows []struct {
		PermissionPoint string `json:"permission_point"`
	}
	err := dao.SysAdminRolePermissions.Ctx(ctx).
		Where("role_id", roleID).
		Fields("permission_point").
		OrderAsc("permission_point").
		Scan(&rows)
	if err = lcommon.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.PermissionPoint
	}
	return out, nil
}

// replaceRolePermissions 全量覆盖角色权限（调用方负责事务与缓存失效）。
func replaceRolePermissions(ctx context.Context, roleID int64, perms []string) error {
	if _, err := dao.SysAdminRolePermissions.Ctx(ctx).
		Where("role_id", roleID).Delete(); err != nil {
		return err
	}
	if len(perms) == 0 {
		return nil
	}
	data := make([]do.SysAdminRolePermissions, len(perms))
	for i, p := range perms {
		data[i] = do.SysAdminRolePermissions{RoleId: roleID, PermissionPoint: p}
	}
	_, err := dao.SysAdminRolePermissions.Ctx(ctx).Data(data).Insert()
	return err
}

// sanitizeGrantedPermissions 校验并规整待授予的权限点：
//  1. 去重、去空、排序，保证同一份权限集合在库里的表示是稳定的
//  2. 拒绝未定义的权限点 —— 写错一个字（channel:veiw）不会有编译期报错，
//     但会让该模块对该角色永久失效，且排查时毫无线索
//  3. 防自我提权：非超管不得授予自己当前不具备的权限点
//
// 第 3 条是兜底。按预置矩阵只有超管拥有 user:edit，但自定义角色可能把 user:* 授予他人，
// 那时若不拦截，一个有 user:edit 的账号就能给自己加满权限，角色体系形同虚设。
func sanitizeGrantedPermissions(ctx context.Context, perms []string) ([]string, error) {
	valid := buildValidPermissionSet()

	seen := make(map[string]bool, len(perms))
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		if !valid[p] {
			return nil, lcommon.NewBadRequestError("无效的权限点：" + p)
		}
		seen[p] = true
		out = append(out, p)
	}
	sort.Strings(out)

	operatorRole := lcommon.GetCtxUserRole(ctx)
	if operatorRole == "super_admin" {
		return out, nil
	}

	operatorID := lcommon.GetCtxUserID(ctx)
	if operatorID == 0 {
		// 没有调用者身份（如后台任务）时不做提权判断，交由上层接口鉴权把关
		return out, nil
	}
	own, err := GetEffectivePermissions(ctx, operatorID, operatorRole)
	if err != nil {
		return nil, err
	}
	ownSet := make(map[string]bool, len(own))
	for _, p := range own {
		ownSet[p] = true
	}
	for _, p := range out {
		if !ownSet[p] {
			return nil, lcommon.NewBadRequestError("不能授予自己不具备的权限：" + p)
		}
	}
	return out, nil
}
