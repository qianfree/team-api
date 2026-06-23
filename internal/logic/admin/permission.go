package admin

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	do "github.com/qianfree/team-api/internal/model/do"

	"github.com/gogf/gf/v2/errors/gerror"
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
			"audit:view", "audit:export", "audit:read_sensitive",
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
			"system:view", "system:edit", "system:update", "system:plugin",
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

	err = dao.SysAdminUsers.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing permissions
		_, err := tx.Model("sys_admin_role_perms").Ctx(ctx).
			Where("admin_user_id", req.Id).
			Delete()
		if err != nil {
			return err
		}

		// Validate permission points against predefined set
		validPerms := buildValidPermissionSet()
		for _, p := range req.Permissions {
			if !validPerms[p] {
				return gerror.Newf("无效的权限点: %s", p)
			}
		}

		// Insert new permissions
		if len(req.Permissions) > 0 {
			data := make([]do.SysAdminRolePerms, len(req.Permissions))
			for i, p := range req.Permissions {
				data[i] = do.SysAdminRolePerms{
					AdminUserId:     req.Id,
					PermissionPoint: p,
				}
			}
			_, err = tx.Model("sys_admin_role_perms").Ctx(ctx).Data(data).Insert()
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

	err = dao.SysAdminUsers.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Delete existing data scopes
		_, err := tx.Model("sys_admin_data_scopes").Ctx(ctx).
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
			_, err = tx.Model("sys_admin_data_scopes").Ctx(ctx).Data(data).Insert()
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

// GetAllPermissions returns all predefined permission groups.
func (s *sAdmin) GetAllPermissions(ctx context.Context, _ *v1.AdminAllPermissionsReq) (*v1.AdminAllPermissionsRes, error) {
	return &v1.AdminAllPermissionsRes{
		Groups: predefinedPermissionGroups,
	}, nil
}

// HasPermission checks if an admin user has a specific permission point.
// super_admin always returns true.
func HasPermission(ctx context.Context, userID int64, role string, permission string) bool {
	if role == "super_admin" {
		return true
	}

	count, err := dao.SysAdminRolePerms.Ctx(ctx).
		Where("admin_user_id", userID).
		Where("permission_point", permission).
		Count()
	if err != nil {
		return false
	}

	return count > 0
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
