package admin

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/frame/g"
	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
)

// predefinedPermissionGroups defines all available permission points grouped by module.
var predefinedPermissionGroups = []v1.PermissionGroup{
	{
		Name:  "tenant",
		Label: "租户管理",
		Permissions: []string{
			"tenant:view", "tenant:create", "tenant:edit", "tenant:delete",
			"tenant:suspend", "tenant:close",
		},
	},
	{
		Name:  "user",
		Label: "用户管理",
		Permissions: []string{
			"user:view", "user:create", "user:edit", "user:delete",
		},
	},
	{
		Name:  "channel",
		Label: "渠道管理",
		Permissions: []string{
			"channel:view", "channel:create", "channel:edit", "channel:delete",
			"channel:test",
		},
	},
	{
		Name:  "model",
		Label: "模型管理",
		Permissions: []string{
			"model:view", "model:create", "model:edit", "model:delete",
		},
	},
	{
		Name:  "billing",
		Label: "计费管理",
		Permissions: []string{
			"billing:view", "billing:export", "billing:refund",
		},
	},
	{
		Name:  "plan",
		Label: "套餐管理",
		Permissions: []string{
			"plan:view", "plan:create", "plan:edit", "plan:delete",
		},
	},
	{
		Name:  "order",
		Label: "订单管理",
		Permissions: []string{
			"order:view", "order:refund",
		},
	},
	{
		Name:  "audit",
		Label: "审计日志",
		Permissions: []string{
			"audit:view", "audit:export", "audit:read_sensitive", "audit:clear",
		},
	},
	{
		Name:  "operation",
		Label: "内容运营",
		Permissions: []string{
			"operation:view", "operation:edit",
		},
	},
	{
		Name:  "support",
		Label: "客户支持",
		Permissions: []string{
			"support:view", "support:reply", "support:edit",
		},
	},
	{
		Name:  "monitor",
		Label: "监控告警",
		Permissions: []string{
			"monitor:view", "monitor:edit",
		},
	},
	{
		Name:  "system",
		Label: "系统设置",
		Permissions: []string{
			// 注：在线自更新无权限点 —— 整个 update 域硬性限定超管（superAdminOnlyRules）
			"system:view", "system:edit", "system:plugin",
		},
	},
	{
		Name:  "dashboard",
		Label: "仪表盘",
		Permissions: []string{
			"dashboard:view",
		},
	},
	{
		Name:  "task",
		Label: "任务管理",
		Permissions: []string{
			"task:view", "task:edit",
		},
	},
	{
		Name:  "promo",
		Label: "优惠码管理",
		Permissions: []string{
			"promo:view", "promo:create", "promo:edit",
		},
	},
	{
		Name:  "invoice",
		Label: "发票管理",
		Permissions: []string{
			"invoice:view", "invoice:manage",
		},
	},
	{
		Name:  "member",
		Label: "成员管理",
		Permissions: []string{
			"member:view", "member:import", "member:model_scope",
		},
	},
	{
		Name:  "redemption",
		Label: "兑换码管理",
		Permissions: []string{
			"redemption:view", "redemption:create", "redemption:edit",
		},
	},
	{
		Name:  "file",
		Label: "文件管理",
		Permissions: []string{
			"file:view", "file:delete", "file:cleanup",
		},
	},
}

// PermissionGroups 返回全部预定义权限点分组。
// 供 middleware 的权限规则覆盖测试校验「规则引用的权限点确实存在」，
// 避免规则里的拼写错误造成接口对所有非超管角色永久 403。
func PermissionGroups() []v1.PermissionGroup {
	return predefinedPermissionGroups
}

// GetUserPermissions returns permission points and data scopes for an admin user.
func (s *sAdmin) GetUserPermissions(ctx context.Context, req *v1.AdminPermissionListReq) (*v1.AdminPermissionListRes, error) {
	// Get permission points
	var perms []struct {
		PermissionPoint string `json:"permission_point"`
	}
	err := dao.SysAdminRolePerms.Ctx(ctx).
		Where("admin_user_id", req.Id).
		Scan(&perms)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	permissions := make([]string, len(perms))
	for i, p := range perms {
		permissions[i] = p.PermissionPoint
	}

	// Get data scopes
	var scopes []struct {
		ID         int64  `json:"id"`
		ScopeType  string `json:"scope_type"`
		ScopeValue string `json:"scope_value"`
	}
	err = dao.SysAdminDataScopes.Ctx(ctx).
		Where("admin_user_id", req.Id).
		Scan(&scopes)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	dataScopes := make([]v1.DataScopeItem, len(scopes))
	for i, sc := range scopes {
		dataScopes[i] = v1.DataScopeItem{
			ID:         sc.ID,
			ScopeType:  sc.ScopeType,
			ScopeValue: sc.ScopeValue,
		}
	}

	return &v1.AdminPermissionListRes{
		Permissions: permissions,
		DataScopes:  dataScopes,
	}, nil
}

// UpdateUserPermissions updates permission points for an admin user.
func (s *sAdmin) UpdateUserPermissions(ctx context.Context, req *v1.AdminPermissionUpdateReq) (*v1.AdminPermissionUpdateRes, error) {
	// Check if target is super_admin
	var user *struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", req.Id).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("用户")
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("超级管理员无需配置权限")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, user.Role); err != nil {
		return nil, err
	}

	// 特批权限与角色权限一样进有效权限的并集，因此必须走同一道 sanitize：
	// 校验权限点存在 + 去重排序 + 禁止授予自己不具备的权限。
	// 此前这里只做「权限点是否存在」的校验，一个拿到 user:edit 的账号可以给自己
	// 特批 system:plugin / billing:refund，绕开「角色管理仅超管」的全部管控。
	perms, err := sanitizeGrantedPermissions(ctx, req.Permissions)
	if err != nil {
		return nil, err
	}

	// 事务采用 ctx 传播式写法：闭包内统一使用 dao.Xxx.Ctx(ctx)，事务由 ctx 自动挂载。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing permissions
		_, err := dao.SysAdminRolePerms.Ctx(ctx).
			Where("admin_user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		// Insert new permissions
		if len(perms) > 0 {
			data := make([]do.SysAdminRolePerms, len(perms))
			for i, p := range perms {
				data[i] = do.SysAdminRolePerms{
					AdminUserId:     req.Id,
					PermissionPoint: p,
				}
			}
			_, err = dao.SysAdminRolePerms.Ctx(ctx).Data(data).Insert()
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// 特批权限参与有效权限计算，变更后必须让该用户的权限缓存失效
	InvalidateUserPermCache(ctx, req.Id)

	return nil, nil
}

// UpdateUserDataScopes updates data scopes for an admin user.
func (s *sAdmin) UpdateUserDataScopes(ctx context.Context, req *v1.AdminDataScopeUpdateReq) (*v1.AdminDataScopeUpdateRes, error) {
	// Check if target is super_admin
	var user *struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", req.Id).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("用户")
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("超级管理员无需配置数据范围")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, user.Role); err != nil {
		return nil, err
	}

	// 事务采用 ctx 传播式写法：闭包内统一使用 dao.Xxx.Ctx(ctx)，事务由 ctx 自动挂载。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing data scopes
		_, err := dao.SysAdminDataScopes.Ctx(ctx).
			Where("admin_user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		// Insert new data scopes
		if len(req.DataScopes) > 0 {
			data := make([]do.SysAdminDataScopes, len(req.DataScopes))
			for i, sc := range req.DataScopes {
				data[i] = do.SysAdminDataScopes{
					AdminUserId: req.Id,
					ScopeType:   sc.ScopeType,
					ScopeValue:  sc.ScopeValue,
				}
			}
			_, err = dao.SysAdminDataScopes.Ctx(ctx).Data(data).Insert()
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// tierLabels 是档位的中文标签，随元数据下发给前端，避免前端硬编码一份。
var tierLabels = map[string]string{
	TierNone:    "无",
	TierRead:    "只读",
	TierOperate: "操作",
	TierFull:    "完全",
}

// GetAllPermissions returns all predefined permission groups plus tier metadata.
//
// 同时返回权限点分组（高级模式用）与「模块 × 档位」元数据（默认配置界面用）：
// 二者是同一份数据的两个视图，由后端统一给出，避免前端复刻一份档位定义造成漂移。
func (s *sAdmin) GetAllPermissions(ctx context.Context, _ *v1.AdminAllPermissionsReq) (*v1.AdminAllPermissionsRes, error) {
	modules := make([]v1.PermissionModuleMeta, 0, len(predefinedPermissionGroups))
	for _, grp := range predefinedPermissionGroups {
		tiers := ModuleTiers(grp.Name)
		opts := make([]v1.TierOption, 0, len(tiers))
		for _, tier := range tiers {
			opts = append(opts, v1.TierOption{
				Tier:        tier,
				Label:       tierLabels[tier],
				Permissions: TierPermissions(grp.Name, tier),
			})
		}
		modules = append(modules, v1.PermissionModuleMeta{
			Module: grp.Name,
			Label:  grp.Label,
			Tiers:  opts,
		})
	}

	return &v1.AdminAllPermissionsRes{
		Groups:    predefinedPermissionGroups,
		Modules:   modules,
		Dangerous: dangerousPermissions,
	}, nil
}

// HasPermission checks if an admin user has a specific permission point.
//
// 有效权限 = ∪(已启用角色的权限) ∪ 用户特批权限，详见 GetEffectivePermissions。
// 结果走 300s 双层缓存：鉴权在每个请求上都跑，此前每次一条 COUNT 查询。
//
// 签名保持不变，middleware/rbac.go 无需改动。
// super_admin 直接放行，不查表也不进缓存。
func HasPermission(ctx context.Context, userID int64, role string, permission string) bool {
	if role == "super_admin" {
		return true
	}

	perms, err := GetEffectivePermissions(ctx, userID, role)
	if err != nil {
		// 查询失败时 fail-closed：宁可误拒也不误放，鉴权失败不应静默放行
		return false
	}
	for _, p := range perms {
		if p == permission {
			return true
		}
	}
	return false
}

// GetDataScopes returns data scopes for an admin user.
// super_admin always returns "all".
func GetDataScopes(ctx context.Context, userID int64, role string) ([]v1.DataScopeItem, error) {
	if role == "super_admin" {
		return []v1.DataScopeItem{{ScopeType: "all"}}, nil
	}

	var scopes []struct {
		ID         int64  `json:"id"`
		ScopeType  string `json:"scope_type"`
		ScopeValue string `json:"scope_value"`
	}
	err := dao.SysAdminDataScopes.Ctx(ctx).
		Where("admin_user_id", userID).
		Scan(&scopes)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	result := make([]v1.DataScopeItem, len(scopes))
	for i, sc := range scopes {
		result[i] = v1.DataScopeItem{
			ID:         sc.ID,
			ScopeType:  sc.ScopeType,
			ScopeValue: sc.ScopeValue,
		}
	}
	return result, nil
}

// buildValidPermissionSet returns a set of all valid permission points.
func buildValidPermissionSet() map[string]bool {
	set := make(map[string]bool)
	for _, g := range predefinedPermissionGroups {
		for _, p := range g.Permissions {
			set[p] = true
		}
	}
	return set
}
