package admin

import (
	"context"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// ListUsers returns a paginated list of admin users.
func (s *sAdmin) ListUsers(ctx context.Context, req *v1.AdminUserListReq) (*v1.AdminUserListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	total, err := buildUserFilters(dao.SysAdminUsers.Ctx(ctx), req.Keyword, req.Role, req.Status).Count()
	if err != nil {
		return nil, err
	}

	m := buildUserFilters(dao.SysAdminUsers.Ctx(ctx), req.Keyword, req.Role, req.Status)

	var users []struct {
		Id          int64       `json:"id"`
		Username    string      `json:"username"`
		Email       string      `json:"email"`
		DisplayName string      `json:"display_name"`
		Role        string      `json:"role"`
		Status      string      `json:"status"`
		LastLoginAt *gtime.Time `json:"last_login_at"`
		LastLoginIp string      `json:"last_login_ip"`
		LockedUntil *gtime.Time `json:"locked_until"`
		CreatedAt   *gtime.Time `json:"created_at"`
	}
	err = m.OrderDesc("id").
		Page(page, pageSize).
		Scan(&users)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	// 批量取角色，避免逐个账号各查一次（N+1）
	userIDs := make([]int64, len(users))
	for i, u := range users {
		userIDs[i] = u.Id
	}
	rolesByUser, err := loadRolesByUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	items := make([]v1.AdminUserItem, len(users))
	for i, u := range users {
		lockedUntil := ""
		if u.LockedUntil != nil {
			lockedUntil = u.LockedUntil.String()
		}
		items[i] = v1.AdminUserItem{
			ID:          u.Id,
			Username:    u.Username,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			Status:      u.Status,
			LastLoginAt: u.LastLoginAt.String(),
			LastLoginIp: u.LastLoginIp,
			LockedUntil: lockedUntil,
			CreatedAt:   u.CreatedAt.String(),
			Roles:       rolesByUser[u.Id],
		}
	}

	return &v1.AdminUserListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// assertCanManageAdminUser 校验当前调用者是否有资格操作目标管理员账号。
//
// 账号管理（改资料 / 改密 / 禁用 / 删除 / 配特批权限）按 user:* 授权、允许下放，
// 但下放的前提是「操作本身不能带来提权」。两条底线：
//   - 目标是超级管理员 → 非超管一律不可操作。否则重置一次超管密码就能接管整个系统，
//     角色管理限定超管的那道闸门形同虚设。
//   - 目标的有效权限不是调用者的子集 → 不可操作。否则一个只有 user:edit 的账号可以
//     重置权限更高账号的密码，登进去就完成了横向提权。
//
// 超级管理员短路放行；操作自己不受限（自身权限的变更另有 sanitizeGrantedPermissions 把关）。
// 与 sanitizeGrantedPermissions 的「不能授予自己不具备的权限」同源：能碰的对象不得强于自己。
func assertCanManageAdminUser(ctx context.Context, targetUserID int64, targetRole string) error {
	operatorRole := common.GetCtxUserRole(ctx)
	if operatorRole == "super_admin" {
		return nil
	}
	if targetRole == "super_admin" {
		return common.NewBusinessError(consts.CodeForbidden, "不能操作超级管理员账号")
	}

	operatorID := common.GetCtxUserID(ctx)
	// 无调用者身份（命令行工具、内部调用）时不做判定，交由上层接口鉴权把关
	if operatorID == 0 || operatorID == targetUserID {
		return nil
	}

	targetPerms, err := GetEffectivePermissions(ctx, targetUserID, targetRole)
	if err != nil {
		return err
	}
	ownPerms, err := GetEffectivePermissions(ctx, operatorID, operatorRole)
	if err != nil {
		return err
	}
	ownSet := make(map[string]bool, len(ownPerms))
	for _, p := range ownPerms {
		ownSet[p] = true
	}
	for _, p := range targetPerms {
		if !ownSet[p] {
			return common.NewBusinessError(consts.CodeForbidden, "目标账号拥有你不具备的权限，无法对其执行此操作")
		}
	}
	return nil
}

// assertCanGrantSuperAdmin 拦截「非超管把账号设成超级管理员」。
//
// role 字段的取值校验（ValidateAdminRole）只管值合不合法，管不了谁有资格赋这个值。
// 少了这道判断，一个拿到 user:create / user:edit 的账号可以直接把自己或新建账号
// 提成 super_admin —— 这是比分配角色更直接的提权入口，必须与角色分配同级管控。
func assertCanGrantSuperAdmin(ctx context.Context, role string) error {
	if role != "super_admin" || common.GetCtxUserRole(ctx) == "super_admin" {
		return nil
	}
	return common.NewBusinessError(consts.CodeForbidden, "只有超级管理员才能设置超级管理员账号")
}

// CreateUser creates a new admin user.
func (s *sAdmin) CreateUser(ctx context.Context, req *v1.AdminUserCreateReq) (*v1.AdminUserCreateRes, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.ToLower(strings.TrimSpace(req.Email))

	// Validate username format
	if err := common.ValidateUsername(username); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidUsername, err.Error())
	}

	// Check username uniqueness
	count, err := dao.SysAdminUsers.Ctx(ctx).
		Where("username", username).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeUsernameExists, consts.MsgUsernameExists)
	}

	// Check email uniqueness if provided (normalized to lowercase so that
	// e.g. "User@Example.com" and "user@example.com" collide as the same address).
	if email != "" {
		count, err = dao.SysAdminUsers.Ctx(ctx).
			Where("email", email).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
		}
	}

	// Validate password policy
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	// Hash password
	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Validate role
	role := req.Role
	if role == "" {
		role = "admin"
	}
	if err := common.ValidateAdminRole(role); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}
	if err := assertCanGrantSuperAdmin(ctx, role); err != nil {
		return nil, err
	}

	// 创建账号时附带角色属于角色分配，与角色管理同级：仅超管可用。
	// 否则拥有 user:create 的人可以造一个挂着高权限角色、密码由自己设定的账号，
	// 等价于给自己提权。这里明确报错而非静默忽略——静默会造出一个零权限账号，
	// 使用者以为分配成功了。
	if len(req.RoleIDs) > 0 && common.GetCtxUserRole(ctx) != "super_admin" {
		return nil, common.NewBusinessError(consts.CodeForbidden, "分配角色仅超级管理员可用，请先创建账号再由超级管理员分配角色")
	}

	// 账号与角色关联同事务写入：此前新建的 admin 账号不带任何权限，登录后处处 403，
	// 是「系统缺乏角色管理」最直接的体感来源。
	var id int64
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		result, err := dao.SysAdminUsers.Ctx(ctx).Data(do.SysAdminUsers{
			Username:     username,
			PasswordHash: passwordHash,
			Email:        email,
			DisplayName:  username,
			Role:         role,
			Status:       "active",
		}).Insert()
		if err != nil {
			// Race condition: another request inserted a colliding username/email
			// between our pre-check and this insert. Translate the DB unique
			// violation into a friendly business error.
			if common.IsDuplicateKeyError(err) {
				return common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
			}
			return err
		}

		id, err = result.LastInsertId()
		if err != nil {
			return err
		}

		// 超级管理员权限来自账号属性而非角色，忽略传入的 role_ids
		if role == "super_admin" || len(req.RoleIDs) == 0 {
			return nil
		}
		return replaceUserRoles(ctx, id, req.RoleIDs)
	})
	if err != nil {
		return nil, err
	}

	return &v1.AdminUserCreateRes{ID: id}, nil
}

// UpdateUser updates an admin user.
func (s *sAdmin) UpdateUser(ctx context.Context, req *v1.AdminUserUpdateReq) (*v1.AdminUserUpdateRes, error) {
	// Prevent modification of super_admin accounts
	var targetUser *struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", req.Id).Scan(&targetUser)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, common.NewNotFoundError("管理员")
	}
	if targetUser.Role == "super_admin" {
		return nil, common.NewBadRequestError("不能修改超级管理员信息")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, targetUser.Role); err != nil {
		return nil, err
	}

	data := do.SysAdminUsers{}
	if req.DisplayName != nil {
		data.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*req.Email))
		// Check uniqueness (normalized to lowercase)
		count, err := dao.SysAdminUsers.Ctx(ctx).
			Where("email", email).Where("id != ?", req.Id).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
		}
		data.Email = email
	}
	if req.Role != nil {
		if err := common.ValidateAdminRole(*req.Role); err != nil {
			return nil, common.NewBadRequestError("角色无效")
		}
		if err := assertCanGrantSuperAdmin(ctx, *req.Role); err != nil {
			return nil, err
		}
		data.Role = *req.Role
	}

	_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Update(data)
	if err != nil {
		if common.IsDuplicateKeyError(err) {
			return nil, common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
		}
		return nil, err
	}

	return nil, nil
}

// DeleteUser deletes an admin user.
func (s *sAdmin) DeleteUser(ctx context.Context, req *v1.AdminUserDeleteReq) (*v1.AdminUserDeleteRes, error) {
	currentUserID := common.GetCtxUserID(ctx)

	if req.Id == currentUserID {
		return nil, common.NewBadRequestError("不能删除当前登录的用户")
	}

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
		return nil, common.NewNotFoundError("管理员")
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("不能删除超级管理员")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, user.Role); err != nil {
		return nil, err
	}

	// Delete user first, then revoke sessions to avoid leaving the user
	// in a state where sessions are gone but the user still exists.
	//
	// 权限相关记录同事务清理：这些表都没有外键，数据库不会代劳，
	// 漏删会留下悬空的角色关联与特批权限。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Delete(); err != nil {
			return err
		}
		if _, err := dao.SysAdminUserRoles.Ctx(ctx).Where("admin_user_id", req.Id).Delete(); err != nil {
			return err
		}
		if _, err := dao.SysAdminRolePerms.Ctx(ctx).Where("admin_user_id", req.Id).Delete(); err != nil {
			return err
		}
		_, err := dao.SysAdminDataScopes.Ctx(ctx).Where("admin_user_id", req.Id).Delete()
		return err
	})
	if err != nil {
		return nil, err
	}

	InvalidateUserPermCache(ctx, req.Id)

	// Revoke all sessions
	common.RevokeAllSessions(ctx, "admin", req.Id)

	return nil, nil
}

// UpdateUserStatus enables or disables an admin user.
func (s *sAdmin) UpdateUserStatus(ctx context.Context, req *v1.AdminUserUpdateStatusReq) (*v1.AdminUserUpdateStatusRes, error) {
	currentUserID := common.GetCtxUserID(ctx)

	if req.Id == currentUserID {
		return nil, common.NewBadRequestError("不能修改当前登录用户的状态")
	}

	if req.Status != "active" && req.Status != "disabled" {
		return nil, common.NewBadRequestError("状态值无效")
	}

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
		return nil, common.NewNotFoundError("管理员")
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("不能修改超级管理员状态")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, user.Role); err != nil {
		return nil, err
	}

	_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Update(do.SysAdminUsers{
		Status: req.Status,
	})
	if err != nil {
		return nil, err
	}

	// If disabling, revoke all sessions
	if req.Status == "disabled" {
		common.RevokeAllSessions(ctx, "admin", req.Id)
	}

	return nil, nil
}

// UnlockUser 清除管理员的登录锁定状态（重置失败计数与锁定截止时间）。
//
// 解锁是本文件里唯一漏掉 assertCanManageAdminUser 的账号处置动作：锁定本身是暴力破解
// 的防线，非超管若能解锁超级管理员账号，就等于可以无限次重置对方的失败计数继续爆破。
func (s *sAdmin) UnlockUser(ctx context.Context, req *v1.AdminUserUnlockReq) (*v1.AdminUserUnlockRes, error) {
	var user *struct {
		Id   int64  `json:"id"`
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Fields("id, role").Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("管理员")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, user.Role); err != nil {
		return nil, err
	}

	_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Data(map[string]interface{}{
		"failed_attempts": 0,
		"locked_until":    nil,
	}).Update()
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ResetUserPassword resets an admin user's password.
func (s *sAdmin) ResetUserPassword(ctx context.Context, req *v1.AdminUserResetPasswordReq) (*v1.AdminUserResetPasswordRes, error) {
	if err := common.ValidatePassword(req.NewPassword); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	// 改密等于账号接管：此前不校验目标是谁，拿到 user:edit 就能重置超级管理员的密码。
	var targetUser *struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Fields("role").Scan(&targetUser)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if targetUser == nil {
		return nil, common.NewNotFoundError("管理员")
	}
	if err := assertCanManageAdminUser(ctx, req.Id, targetUser.Role); err != nil {
		return nil, err
	}

	passwordHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, err
	}

	_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Update(do.SysAdminUsers{
		PasswordHash: passwordHash,
	})
	if err != nil {
		return nil, err
	}

	// Force re-login
	common.RevokeAllSessions(ctx, "admin", req.Id)

	return nil, nil
}

// buildUserFilters builds the WHERE conditions for admin user queries.
func buildUserFilters(m *gdb.Model, keyword, role, status string) *gdb.Model {
	if keyword != "" {
		kw := "%" + strings.TrimSpace(keyword) + "%"
		m = m.Where("username LIKE ? OR email LIKE ?", kw, kw)
	}
	if role != "" {
		m = m.Where("role", role)
	}
	if status != "" {
		m = m.Where("status", status)
	}
	return m
}

// ExportUsers exports admin users to CSV or Excel.
func (s *sAdmin) ExportUsers(ctx context.Context, req *v1.AdminUserExportReq) (*v1.AdminUserExportRes, error) {
	config := export.Config{
		Format:   req.Format,
		Filename: "管理员_" + gtime.Now().Format("Ymd_His"),
		Columns: []export.Column{
			{Field: "id", Header: "ID"},
			{Field: "username", Header: "用户名"},
			{Field: "email", Header: "邮箱"},
			{Field: "display_name", Header: "显示名称"},
			{Field: "role", Header: "角色"},
			{Field: "status", Header: "状态"},
			{Field: "last_login_at", Header: "最后登录时间"},
			{Field: "last_login_ip", Header: "最后登录IP"},
			{Field: "created_at", Header: "创建时间"},
		},
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			m := buildUserFilters(dao.SysAdminUsers.Ctx(ctx), req.Keyword, req.Role, req.Status)
			var batch []struct {
				Id          int64       `json:"id"`
				Username    string      `json:"username"`
				Email       string      `json:"email"`
				DisplayName string      `json:"display_name"`
				Role        string      `json:"role"`
				Status      string      `json:"status"`
				LastLoginAt *gtime.Time `json:"last_login_at"`
				LastLoginIp string      `json:"last_login_ip"`
				CreatedAt   *gtime.Time `json:"created_at"`
			}
			if err := m.OrderDesc("id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, u := range batch {
				if !yield(map[string]any{
					"id":            u.Id,
					"username":      u.Username,
					"email":         u.Email,
					"display_name":  u.DisplayName,
					"role":          u.Role,
					"status":        u.Status,
					"last_login_at": u.LastLoginAt.String(),
					"last_login_ip": u.LastLoginIp,
					"created_at":    u.CreatedAt.String(),
				}) {
					return
				}
			}
			if len(batch) < 1000 {
				break
			}
			offset += 1000
		}
	})
}
