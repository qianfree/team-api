package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"
)

// CreateMember adds a new member to a specified tenant.
func (s *sAdmin) CreateMember(ctx context.Context, req *v1.AdminMemberCreateReq) (*v1.AdminMemberCreateRes, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))
	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	// Validate password
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	var userID int64

	err = dao.TntTenants.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Check tenant exists and status
		var tenant *struct {
			Id     int64  `json:"id"`
			Status string `json:"status"`
		}
		err := tx.Model("tnt_tenants").Ctx(ctx).
			Where("id", req.TenantID).Scan(&tenant)
		if err = common.IgnoreScanNoRows(err); err != nil {
			return err
		}
		if tenant == nil {
			return common.NewNotFoundError("租户")
		}
		if tenant.Status != "active" {
			return common.NewBadRequestError("租户状态异常，无法添加成员")
		}

		// Check member limit（NULL时取等级配置）
		effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, req.TenantID)
		if err != nil {
			return err
		}
		memberCount, err := tx.Model("tnt_users").Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("status", "active").
			Count()
		if err != nil {
			return err
		}
		if memberCount >= effectiveMaxMembers {
			return common.NewBusinessError(consts.CodeMemberLimitReached, consts.MsgMemberLimitReached)
		}

		// Check username uniqueness within tenant
		count, err := tx.Model("tnt_users").Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("username", username).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return common.NewBusinessError(consts.CodeUsernameExists, consts.MsgUsernameExists)
		}

		// Check email uniqueness within tenant
		count, err = tx.Model("tnt_users").Ctx(ctx).
			Where("tenant_id", req.TenantID).
			Where("email", email).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return common.NewBadRequestError("该邮箱已被使用")
		}

		// Create user
		userResult, err := tx.Model("tnt_users").Ctx(ctx).Data(do.TntUsers{
			TenantId:     req.TenantID,
			Username:     username,
			Email:        email,
			PasswordHash: passwordHash,
			DisplayName:  displayName,
			Role:         req.Role,
			Status:       "active",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "创建用户失败")
		}
		userID, err = userResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "获取用户ID失败")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &v1.AdminMemberCreateRes{Id: userID}, nil
}

// ListAllMembers returns a paginated list of all tenant members across all tenants.
func (s *sAdmin) ListAllMembers(ctx context.Context, req *v1.AdminMemberListReq) (*v1.AdminMemberListRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	m := dao.TntUsers.Ctx(ctx).
		LeftJoin("tnt_tenants t ON tnt_users.tenant_id = t.id")
	m = buildMemberFilters(m, req.Keyword, req.Status, req.Role, req.TenantID)

	total, err := m.Count()
	if err != nil {
		return nil, err
	}

	// Rebuild model for data query (Count() modifies internal state)
	m = dao.TntUsers.Ctx(ctx).
		LeftJoin("tnt_tenants t ON tnt_users.tenant_id = t.id")
	m = buildMemberFilters(m, req.Keyword, req.Status, req.Role, req.TenantID)

	var members []struct {
		Id             int64       `json:"id"`
		TenantId       int64       `json:"tenant_id"`
		TenantName     string      `json:"tenant_name"`
		TenantCode     string      `json:"tenant_code"`
		Username       string      `json:"username"`
		Email          string      `json:"email"`
		DisplayName    string      `json:"display_name"`
		Role           string      `json:"role"`
		Status         string      `json:"status"`
		LastLoginAt    *gtime.Time `json:"last_login_at"`
		LastLoginIp    string      `json:"last_login_ip"`
		FailedAttempts int         `json:"failed_attempts"`
		CreatedAt      *gtime.Time `json:"created_at"`
	}
	err = m.Fields("tnt_users.id, tnt_users.tenant_id, t.name as tenant_name, t.code as tenant_code, tnt_users.username, tnt_users.email, tnt_users.display_name, tnt_users.role, tnt_users.status, tnt_users.last_login_at, tnt_users.last_login_ip, tnt_users.failed_attempts, tnt_users.created_at").
		OrderDesc("tnt_users.id").
		Page(page, pageSize).
		Scan(&members)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	items := make([]v1.AdminMemberItem, len(members))
	for i, m := range members {
		items[i] = v1.AdminMemberItem{
			ID:             m.Id,
			TenantID:       m.TenantId,
			TenantName:     m.TenantName,
			TenantCode:     m.TenantCode,
			Username:       m.Username,
			Email:          m.Email,
			DisplayName:    m.DisplayName,
			Role:           m.Role,
			Status:         m.Status,
			LastLoginAt:    m.LastLoginAt.String(),
			LastLoginIP:    m.LastLoginIp,
			FailedAttempts: m.FailedAttempts,
			CreatedAt:      m.CreatedAt.String(),
		}
	}

	return &v1.AdminMemberListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// DisableMember disables a tenant member by admin.
func (s *sAdmin) DisableMember(ctx context.Context, req *v1.AdminMemberDisableReq) (*v1.AdminMemberDisableRes, error) {
	var user *struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}
	if user.Status == "disabled" {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该成员已是禁用状态")
	}

	// Revoke all active API keys and disable user in transaction
	err = dao.TntUsers.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("api_keys").Ctx(ctx).
			Where("user_id", req.Id).
			Where("status", "active").
			Data(do.ApiKeys{
				Status: "disabled",
			}).Update()
		if err != nil {
			return err
		}

		_, err = tx.Model("tnt_users").Ctx(ctx).
			Where("id", req.Id).
			Data(do.TntUsers{
				Status: "disabled",
			}).Update()
		return err
	})
	if err != nil {
		return nil, err
	}
	return &v1.AdminMemberDisableRes{}, nil
}

// EnableMember re-enables a tenant member by admin.
func (s *sAdmin) EnableMember(ctx context.Context, req *v1.AdminMemberEnableReq) (*v1.AdminMemberEnableRes, error) {
	var user *struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}
	if user.Status != "disabled" {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "该成员不是禁用状态")
	}

	// Restore disabled API keys and enable user in transaction
	err = dao.TntUsers.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, err := tx.Model("api_keys").Ctx(ctx).
			Where("user_id", req.Id).
			Where("status", "disabled").
			Data(do.ApiKeys{
				Status: "active",
			}).Update()
		if err != nil {
			return err
		}

		_, err = tx.Model("tnt_users").Ctx(ctx).
			Where("id", req.Id).
			Data(do.TntUsers{
				Status: "active",
			}).Update()
		return err
	})
	if err != nil {
		return nil, err
	}
	return &v1.AdminMemberEnableRes{}, nil
}

// ResetMemberPassword resets a member's password by admin, returns the new random password.
func (s *sAdmin) ResetMemberPassword(ctx context.Context, req *v1.AdminMemberResetPasswordReq) (*v1.AdminMemberResetPasswordRes, error) {
	var user *struct {
		Status string `json:"status"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}

	// Generate random password
	passwordBytes := make([]byte, 16)
	if _, err := rand.Read(passwordBytes); err != nil {
		return nil, err
	}
	newPassword := hex.EncodeToString(passwordBytes)[:12]

	passwordHash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}

	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Data(do.TntUsers{
			PasswordHash:   passwordHash,
			FailedAttempts: 0,
			LockedUntil:    nil,
		}).Update()
	if err != nil {
		return nil, err
	}

	return &v1.AdminMemberResetPasswordRes{NewPassword: newPassword}, nil
}

// buildMemberFilters builds the WHERE conditions for member queries.
func buildMemberFilters(m *gdb.Model, keyword, status, role string, tenantID int64) *gdb.Model {
	if keyword != "" {
		kw := "%" + strings.TrimSpace(keyword) + "%"
		m = m.Where("tnt_users.username LIKE ? OR tnt_users.email LIKE ? OR tnt_users.display_name LIKE ?", kw, kw, kw)
	}
	if status != "" {
		m = m.Where("tnt_users.status", status)
	}
	if role != "" {
		m = m.Where("tnt_users.role", role)
	}
	if tenantID > 0 {
		m = m.Where("tnt_users.tenant_id", tenantID)
	}
	return m
}

// ExportMembers exports member list to CSV or Excel.
func (s *sAdmin) ExportMembers(ctx context.Context, req *v1.AdminMemberExportReq) (*v1.AdminMemberExportRes, error) {
	memberFields := "tnt_users.id, tnt_users.tenant_id, t.name as tenant_name, t.code as tenant_code, tnt_users.username, tnt_users.email, tnt_users.display_name, tnt_users.role, tnt_users.status, tnt_users.last_login_at, tnt_users.last_login_ip, tnt_users.failed_attempts, tnt_users.created_at"

	config := export.Config{
		Format:   req.Format,
		Filename: "成员_" + gtime.Now().Format("Ymd_His"),
		Columns: []export.Column{
			{Field: "id", Header: "ID"},
			{Field: "tenant_name", Header: "租户名称"},
			{Field: "username", Header: "用户名"},
			{Field: "email", Header: "邮箱"},
			{Field: "display_name", Header: "显示名称"},
			{Field: "role", Header: "角色"},
			{Field: "status", Header: "状态"},
			{Field: "last_login_at", Header: "最后登录时间"},
			{Field: "created_at", Header: "创建时间"},
		},
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			m := buildMemberFilters(
				dao.TntUsers.Ctx(ctx).LeftJoin("tnt_tenants t ON tnt_users.tenant_id = t.id"),
				req.Keyword, req.Status, req.Role, req.TenantID,
			)
			var batch []struct {
				Id          int64       `json:"id"`
				TenantName  string      `json:"tenant_name"`
				Username    string      `json:"username"`
				Email       string      `json:"email"`
				DisplayName string      `json:"display_name"`
				Role        string      `json:"role"`
				Status      string      `json:"status"`
				LastLoginAt *gtime.Time `json:"last_login_at"`
				CreatedAt   *gtime.Time `json:"created_at"`
			}
			if err := m.Fields(memberFields).OrderDesc("tnt_users.id").Limit(1000).Offset(offset).Scan(&batch); err != nil {
				return
			}
			for _, m := range batch {
				if !yield(map[string]any{
					"id":            m.Id,
					"tenant_name":   m.TenantName,
					"username":      m.Username,
					"email":         m.Email,
					"display_name":  m.DisplayName,
					"role":          m.Role,
					"status":        m.Status,
					"last_login_at": m.LastLoginAt.String(),
					"created_at":    m.CreatedAt.String(),
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
