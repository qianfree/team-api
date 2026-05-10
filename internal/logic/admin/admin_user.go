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

	m := dao.SysAdminUsers.Ctx(ctx)

	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
	}
	if req.Role != "" {
		m = m.Where("role", req.Role)
	}
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	// Rebuild model for data query (Count() modifies internal state)
	m = dao.SysAdminUsers.Ctx(ctx)
	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		m = m.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
	}
	if req.Role != "" {
		m = m.Where("role", req.Role)
	}
	if req.Status != "" {
		m = m.Where("status", req.Status)
	}

	var users []struct {
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
	err = m.OrderDesc("id").
		Page(page, pageSize).
		Scan(&users)
	if err != nil {
		return nil, err
	}

	items := make([]v1.AdminUserItem, len(users))
	for i, u := range users {
		items[i] = v1.AdminUserItem{
			ID:          u.Id,
			Username:    u.Username,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			Status:      u.Status,
			LastLoginAt: u.LastLoginAt.String(),
			LastLoginIp: u.LastLoginIp,
			CreatedAt:   u.CreatedAt.String(),
		}
	}

	return &v1.AdminUserListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateUser creates a new admin user.
func (s *sAdmin) CreateUser(ctx context.Context, req *v1.AdminUserCreateReq) (*v1.AdminUserCreateRes, error) {
	username := strings.TrimSpace(req.Username)

	// Check username uniqueness
	count, err := dao.SysAdminUsers.Ctx(ctx).
		Where("username", username).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeUsernameExists, consts.MsgUsernameExists)
	}

	// Check email uniqueness if provided
	if req.Email != "" {
		count, err = dao.SysAdminUsers.Ctx(ctx).
			Where("email", strings.TrimSpace(req.Email)).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBadRequestError("邮箱已被使用")
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

	result, err := dao.SysAdminUsers.Ctx(ctx).Data(do.SysAdminUsers{
		Username:     username,
		PasswordHash: passwordHash,
		Email:        strings.TrimSpace(req.Email),
		DisplayName:  username,
		Role:         role,
		Status:       "active",
	}).Insert()
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &v1.AdminUserCreateRes{ID: id}, nil
}

// UpdateUser updates an admin user.
func (s *sAdmin) UpdateUser(ctx context.Context, req *v1.AdminUserUpdateReq) (*v1.AdminUserUpdateRes, error) {
	data := do.SysAdminUsers{}
	if req.DisplayName != nil {
		data.DisplayName = *req.DisplayName
	}
	if req.Email != nil {
		email := strings.TrimSpace(*req.Email)
		// Check uniqueness
		count, err := dao.SysAdminUsers.Ctx(ctx).
			Where("email", email).Where("id != ?", req.Id).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBadRequestError("邮箱已被使用")
		}
		data.Email = email
	}
	if req.Role != nil {
		if err := common.ValidateAdminRole(*req.Role); err != nil {
			return nil, common.NewBadRequestError("角色无效")
		}
		data.Role = *req.Role
	}

	_, err := dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Update(data)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// DeleteUser deletes an admin user.
func (s *sAdmin) DeleteUser(ctx context.Context, req *v1.AdminUserDeleteReq) (*v1.AdminUserDeleteRes, error) {
	currentUserID := ctxUserID(ctx)

	if req.Id == currentUserID {
		return nil, common.NewBadRequestError("不能删除当前登录的用户")
	}

	// Check if target is super_admin
	var user struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", req.Id).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("不能删除超级管理员")
	}

	// Revoke all sessions
	common.RevokeAllSessions(ctx, "admin", req.Id)

	// Delete user
	_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", req.Id).Delete()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// UpdateUserStatus enables or disables an admin user.
func (s *sAdmin) UpdateUserStatus(ctx context.Context, req *v1.AdminUserUpdateStatusReq) (*v1.AdminUserUpdateStatusRes, error) {
	currentUserID := ctxUserID(ctx)

	if req.Id == currentUserID {
		return nil, common.NewBadRequestError("不能修改当前登录用户的状态")
	}

	if req.Status != "active" && req.Status != "disabled" {
		return nil, common.NewBadRequestError("状态值无效")
	}

	// Check if target is super_admin
	var user struct {
		Role string `json:"role"`
	}
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", req.Id).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Role == "super_admin" {
		return nil, common.NewBadRequestError("不能修改超级管理员状态")
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

// ResetUserPassword resets an admin user's password.
func (s *sAdmin) ResetUserPassword(ctx context.Context, req *v1.AdminUserResetPasswordReq) (*v1.AdminUserResetPasswordRes, error) {
	if err := common.ValidatePassword(req.NewPassword); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
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
	r := g.RequestFromCtx(ctx)
	format := req.Format
	if format == "" {
		format = "csv"
	}
	if format == "excel" {
		format = "xlsx"
	}

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "username", Header: "用户名"},
		{Field: "email", Header: "邮箱"},
		{Field: "display_name", Header: "显示名称"},
		{Field: "role", Header: "角色"},
		{Field: "status", Header: "状态"},
		{Field: "last_login_at", Header: "最后登录时间"},
		{Field: "last_login_ip", Header: "最后登录IP"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   format,
		Filename: "管理员_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	if format == "xlsx" {
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
			CreatedAt   *gtime.Time `json:"created_at"`
		}
		if err := m.OrderDesc("id").Scan(&users); err != nil {
			return nil, err
		}
		data := make([]map[string]any, len(users))
		for i, u := range users {
			data[i] = map[string]any{
				"id":            u.Id,
				"username":      u.Username,
				"email":         u.Email,
				"display_name":  u.DisplayName,
				"role":          u.Role,
				"status":        u.Status,
				"last_login_at": u.LastLoginAt.String(),
				"last_login_ip": u.LastLoginIp,
				"created_at":    u.CreatedAt.String(),
			}
		}
		return nil, export.WriteExcel(r, config, data)
	}

	return nil, export.StreamCSV(r, config, func(yield func(map[string]any) bool) {
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
