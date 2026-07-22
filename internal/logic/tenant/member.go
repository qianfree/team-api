package tenant

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/billing"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/export"

	do "github.com/qianfree/team-api/internal/model/do"
)

// ListMembers returns a paginated list of tenant members.
func (s *sTenant) ListMembers(ctx context.Context, req *v1.TenantMemberListReq) (*v1.TenantMemberListRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	model := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID)

	if req.Keyword != "" {
		keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
		model = model.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
	}
	if req.Role != "" {
		model = model.Where("role", req.Role)
	}

	total, err := model.Count()
	if err != nil {
		return nil, err
	}

	var users []struct {
		Id          int64  `json:"id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
	}
	err = model.OrderDesc("id").
		Page(page, pageSize).
		Scan(&users)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	items := make([]v1.TenantMemberItem, len(users))
	for i, u := range users {
		items[i] = v1.TenantMemberItem{
			ID:          u.Id,
			Username:    u.Username,
			Email:       u.Email,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			Status:      u.Status,
			CreatedAt:   u.CreatedAt,
		}
	}

	return &v1.TenantMemberListRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// InviteMember generates an invitation link.
func (s *sTenant) InviteMember(ctx context.Context, req *v1.TenantMemberInviteReq) (*v1.TenantMemberInviteRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)
	creatorID := middleware.GetUserID(ctx)

	// Check member limit
	memberCount, err := dao.TntUsers.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("status", "active").
		Count()
	if err != nil {
		return nil, err
	}

	// 获取实际生效的成员数上限（NULL时取等级配置）
	effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, tenantID)
	if err != nil {
		return nil, gerror.Wrapf(err, "查询租户限制信息失败")
	}

	// effectiveMaxMembers == 0 表示无限制
	if effectiveMaxMembers > 0 && int(memberCount) >= effectiveMaxMembers {
		return nil, common.NewBusinessError(consts.CodeMemberLimitReached, consts.MsgMemberLimitReached)
	}

	// Validate role
	if err := common.ValidateTenantRole(req.Role); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	// Generate invite code
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, gerror.Wrapf(err, "生成邀请码失败")
	}
	code := hex.EncodeToString(codeBytes)

	// Calculate expiry
	expiresDays := req.ExpiresDays
	if expiresDays < 1 {
		expiresDays = 7
	}
	if expiresDays > 30 {
		expiresDays = 30
	}
	expiresAt := time.Now().Add(time.Duration(expiresDays) * 24 * time.Hour)

	_, err = dao.TntInvitations.Ctx(ctx).Data(do.TntInvitations{
		TenantId:  tenantID,
		Code:      code,
		Role:      req.Role,
		ExpiresAt: gtime.NewFromTime(expiresAt),
		MaxUses:   req.MaxUses,
		CreatedBy: creatorID,
	}).Insert()
	if err != nil {
		return nil, gerror.Wrapf(err, "创建邀请失败")
	}

	inviteURL := buildInviteURL(ctx, code)

	return &v1.TenantMemberInviteRes{
		Code:      code,
		InviteURL: inviteURL,
		ExpiresAt: expiresAt.Format(time.RFC3339),
		MaxUses:   req.MaxUses,
	}, nil
}

// buildInviteURL constructs the full invitation URL.
// Priority: sys_options tenant_console_url > Origin header > Host header.
func buildInviteURL(ctx context.Context, code string) string {
	base := strings.TrimRight(common.Config().GetOption(ctx, "tenant_console_url"), "/")
	if base == "" {
		r := g.RequestFromCtx(ctx)
		if r != nil {
			base = r.GetHeader("Origin")
			if base == "" {
				scheme := "https"
				if r.GetHeader("X-Forwarded-Proto") == "http" || strings.HasPrefix(r.GetHost(), "localhost") || strings.HasPrefix(r.GetHost(), "127.0.0.1") {
					scheme = "http"
				}
				base = fmt.Sprintf("%s://%s", scheme, r.GetHost())
			}
		}
	}
	return fmt.Sprintf("%s/#/tenant/join?code=%s", base, code)
}

// nullableEmail converts an empty email to nil so an optional email is stored as
// SQL NULL (tnt_users.email is nullable) rather than "" — the latter would occupy
// the UNIQUE(tenant_id, email) slot and block other email-less members in the tenant.
func nullableEmail(email string) any {
	if email == "" {
		return nil
	}
	return email
}

// JoinByInvite handles a user joining a tenant via invitation link.
func (s *sTenant) JoinByInvite(ctx context.Context, req *v1.TenantMemberJoinReq) (*v1.TenantMemberJoinRes, error) {
	// Find invitation
	var invitation *struct {
		ID           int64       `json:"id"`
		TenantID     int64       `json:"tenant_id"`
		Role         string      `json:"role"`
		ExpiresAt    *gtime.Time `json:"expires_at"`
		MaxUses      int         `json:"max_uses"`
		UseCount     int         `json:"use_count"`
		UsedByUserID int64       `json:"used_by_user_id"`
	}
	err := dao.TntInvitations.Ctx(ctx).
		Where("code", req.Code).
		Scan(&invitation)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if invitation == nil {
		return nil, common.NewBusinessError(consts.CodeInvitationExpired, consts.MsgInvitationExpired)
	}

	// Check if revoked
	if invitation.UsedByUserID == -1 {
		return nil, common.NewBusinessError(consts.CodeInvitationUsed, consts.MsgInvitationUsed)
	}

	// Check if exhausted (max_uses > 0 means limited)
	if invitation.MaxUses > 0 && invitation.UseCount >= invitation.MaxUses {
		return nil, common.NewBusinessError(consts.CodeInvitationUsed, consts.MsgInvitationUsed)
	}

	// Check expiry
	if invitation.ExpiresAt != nil && time.Now().After(invitation.ExpiresAt.Time) {
		return nil, common.NewBusinessError(consts.CodeInvitationExpired, consts.MsgInvitationExpired)
	}

	// 团队功能门控：目标租户必须已启用团队功能（公开接口，ctx 无 tenant_id，用目标租户 ID 校验）
	if err := requireTeamEnabledForTenant(ctx, invitation.TenantID); err != nil {
		return nil, err
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Validate password
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	tenantID := invitation.TenantID

	// Validate username format
	if err := common.ValidateUsername(username); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidUsername, err.Error())
	}

	var userID int64

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Check member limit
		memberCount, err := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("status", "active").
			Count()
		if err != nil {
			return err
		}

		// 获取实际生效的成员数上限（NULL时取等级配置）
		effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, tenantID)
		if err != nil {
			return err
		}
		// effectiveMaxMembers == 0 表示无限制
		if effectiveMaxMembers > 0 && int(memberCount) >= effectiveMaxMembers {
			return common.NewBusinessError(consts.CodeMemberLimitReached, consts.MsgMemberLimitReached)
		}

		// Check username uniqueness within tenant
		count, err := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("username", username).
			Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return common.NewBusinessError(consts.CodeUsernameExists, consts.MsgUsernameExists)
		}

		// Check email uniqueness within tenant (email is optional — skip when empty)
		if email != "" {
			count, err = dao.TntUsers.Ctx(ctx).
				Where("tenant_id", tenantID).
				Where("email", email).
				Count()
			if err != nil {
				return err
			}
			if count > 0 {
				return common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
			}
		}

		// Create user
		displayName := req.DisplayName
		if displayName == "" {
			displayName = username
		}

		userResult, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
			TenantId:     tenantID,
			Username:     username,
			Email:        nullableEmail(email),
			PasswordHash: passwordHash,
			DisplayName:  displayName,
			Role:         invitation.Role,
			Status:       "active",
		}).Insert()
		if err != nil {
			// Race condition: another request inserted a colliding
			// username/email between our pre-check and this insert.
			if common.IsDuplicateKeyError(err) {
				return common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
			}
			return gerror.Wrapf(err, "创建用户失败")
		}
		userID, err = userResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "获取用户ID失败")
		}

		// Increment invitation use count
		_, err = dao.TntInvitations.Ctx(ctx).
			Where("id", invitation.ID).
			Data(do.TntInvitations{
				UseCount: gdb.Raw("use_count + 1"),
				UsedAt:   gtime.Now(),
			}).Update()
		if err != nil {
			return gerror.Wrapf(err, "标记邀请已使用失败")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Generate tokens
	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)

	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	deviceInfo := extractTenantDeviceInfo(ctx)
	jti := common.GenerateJti()
	sessionID, err := common.CreateSession(ctx, "tenant", userID, tenantID, refreshTokenHash, ipAddress, deviceInfo, jti)
	if err != nil {
		return nil, gerror.Wrapf(err, "创建会话失败")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, userID, "tenant", invitation.Role, tenantID, sessionID, jti)
	if err != nil {
		return nil, err
	}

	// Query tenant info for response
	var tenantInfo *struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}
	_ = dao.TntTenants.Ctx(ctx).
		Where("id", tenantID).
		Fields("name", "code").
		Scan(&tenantInfo)

	return &v1.TenantMemberJoinRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
		TenantName:   tenantInfo.Name,
		TenantCode:   tenantInfo.Code,
		Username:     username,
		Role:         invitation.Role,
	}, nil
}

// CreateMember directly creates a member account within the tenant.
func (s *sTenant) CreateMember(ctx context.Context, req *v1.TenantMemberCreateReq) (*v1.TenantMemberCreateRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	// Validate role
	if err := common.ValidateTenantRole(req.Role); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	// Validate password strength
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Validate username format
	if err := common.ValidateUsername(username); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidUsername, err.Error())
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = username
	}

	var userID int64

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Check member limit
		memberCount, err := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("status", "active").
			Count()
		if err != nil {
			return err
		}

		// 获取实际生效的成员数上限（NULL时取等级配置）
		effectiveMaxMembers, _, err := billing.GetTenantEffectiveLimits(ctx, tenantID)
		if err != nil {
			return gerror.Wrapf(err, "查询租户限制信息失败")
		}
		// effectiveMaxMembers == 0 表示无限制
		if effectiveMaxMembers > 0 && int(memberCount) >= effectiveMaxMembers {
			return common.NewBusinessError(consts.CodeMemberLimitReached, consts.MsgMemberLimitReached)
		}

		// Check username uniqueness within tenant
		count, err := dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("username", username).
			Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return common.NewBusinessError(consts.CodeUsernameExists, consts.MsgUsernameExists)
		}

		// Check email uniqueness within tenant (skip if empty)
		if email != "" {
			count, err = dao.TntUsers.Ctx(ctx).
				Where("tenant_id", tenantID).
				Where("email", email).
				Count()
			if err != nil {
				return err
			}
			if count > 0 {
				return common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
			}
		}

		// Create user
		userResult, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
			TenantId:     tenantID,
			Username:     username,
			Email:        nullableEmail(email),
			PasswordHash: passwordHash,
			DisplayName:  displayName,
			Role:         req.Role,
			Status:       "active",
		}).Insert()
		if err != nil {
			// Race condition: another request inserted a colliding
			// username/email between our pre-check and this insert.
			if common.IsDuplicateKeyError(err) {
				return common.NewBusinessError(consts.CodeEmailExists, consts.MsgEmailExists)
			}
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

	return &v1.TenantMemberCreateRes{ID: userID}, nil
}

// RemoveMember removes a member from the tenant.
// Revokes all API keys, anonymizes personal data, releases member model scopes.
func (s *sTenant) RemoveMember(ctx context.Context, req *v1.TenantMemberRemoveReq) (*v1.TenantMemberRemoveRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)
	currentUserID := middleware.GetUserID(ctx)
	memberID := req.Id

	if memberID == currentUserID {
		return nil, common.NewBadRequestError("不能移除自己")
	}

	// Check if target is owner
	var user *struct {
		Role        string `json:"role"`
		DisplayName string `json:"display_name"`
		Username    string `json:"username"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", memberID).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}
	if user.Role == "owner" {
		return nil, common.NewBadRequestError("不能移除组织所有者")
	}

	removedDisplayName := user.DisplayName
	if removedDisplayName == "" {
		removedDisplayName = user.Username
	}
	if removedDisplayName != "" {
		removedDisplayName = fmt.Sprintf("[已移除] %s", removedDisplayName)
	} else {
		removedDisplayName = "[已移除成员]"
	}

	// Revoke all sessions（Redis + DB 会话表）。会话撤销属于尽力而为的安全动作，
	// 放在事务外先行执行：即便后续 DB 事务回滚，也已让该成员的活跃会话失效（fail-safe 方向）。
	common.RevokeAllSessions(ctx, "tenant", memberID)

	// 撤销 API Key → 删除成员模型范围 → 匿名化个人数据，三步写操作放入同一事务，
	// 避免中途失败导致「Key 已撤销但用户未匿名化」等半完成的不一致状态。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Revoke all API keys for this user
		if _, err := dao.ApiKeys.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", memberID).
			Data(do.ApiKeys{
				Status: "revoked",
			}).Update(); err != nil {
			return err
		}

		// Remove member model scopes
		if _, err := dao.TntMemberModelScopes.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", memberID).
			Delete(); err != nil {
			return err
		}

		// Anonymize personal data
		if _, err := dao.TntUsers.Ctx(ctx).
			Where("id", memberID).
			Where("tenant_id", tenantID).
			Data(do.TntUsers{
				Status:      "removed",
				Email:       fmt.Sprintf("deleted_%d@removed.local", memberID),
				DisplayName: removedDisplayName,
				Username:    fmt.Sprintf("deleted_%d", memberID),
			}).Update(); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// DisableMember disables a member and revokes all their API keys.
func (s *sTenant) DisableMember(ctx context.Context, tenantID, userID int64) error {
	var user *struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return err
	}
	if user == nil {
		return common.NewNotFoundError("成员")
	}
	if user.Role == "owner" {
		return common.NewBadRequestError("不能禁用组织所有者")
	}
	if user.Status == "disabled" || user.Status == "removed" {
		return nil // already disabled
	}

	// 撤销 API Key 与更新成员状态放入同一事务，避免「Key 已禁用但成员状态未更新」的不一致。
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Revoke all active API keys
		if _, err := dao.ApiKeys.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", userID).
			Where("status", "active").
			Data(do.ApiKeys{
				Status: "disabled",
			}).Update(); err != nil {
			return err
		}

		_, err := dao.TntUsers.Ctx(ctx).
			Where("id", userID).
			Where("tenant_id", tenantID).
			Data(do.TntUsers{
				Status: "disabled",
			}).Update()
		return err
	})
}

// EnableMember re-enables a member and restores their API keys.
func (s *sTenant) EnableMember(ctx context.Context, tenantID, userID int64) error {
	var user *struct {
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return err
	}
	if user == nil {
		return nil
	}
	if user.Status != "disabled" {
		return nil // already active or removed
	}

	// 恢复 API Key 与更新成员状态放入同一事务，避免「Key 已恢复但成员状态未更新」的不一致。
	return g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Restore disabled API keys
		if _, err := dao.ApiKeys.Ctx(ctx).
			Where("tenant_id", tenantID).
			Where("user_id", userID).
			Where("status", "disabled").
			Data(do.ApiKeys{
				Status: "active",
			}).Update(); err != nil {
			return err
		}

		_, err := dao.TntUsers.Ctx(ctx).
			Where("id", userID).
			Where("tenant_id", tenantID).
			Data(do.TntUsers{
				Status: "active",
			}).Update()
		return err
	})
}

// UpdateMemberRole updates a member's role.
func (s *sTenant) UpdateMemberRole(ctx context.Context, req *v1.TenantMemberUpdateRoleReq) (*v1.TenantMemberUpdateRoleRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)
	currentUserID := middleware.GetUserID(ctx)
	memberID := req.Id

	if memberID == currentUserID {
		return nil, common.NewBadRequestError("不能修改自己的角色")
	}

	if err := common.ValidateTenantRole(req.Role); err != nil {
		return nil, common.NewBadRequestError(err.Error())
	}

	var user *struct {
		Role string `json:"role"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", memberID).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}
	if user.Role == "owner" {
		return nil, common.NewBadRequestError("不能修改所有者的角色")
	}

	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", memberID).
		Where("tenant_id", tenantID).
		Data(do.TntUsers{
			Role: req.Role,
		}).Update()
	if err != nil {
		return nil, err
	}

	// 角色变更后强制下线该成员，使其重新登录以获取新角色的 JWT
	if revokeErr := common.RevokeAllSessions(ctx, "tenant", memberID); revokeErr != nil {
		g.Log().Warningf(ctx, "撤销成员 %d 会话失败: %v", memberID, revokeErr)
	}

	return nil, nil
}

// ResetMemberPassword resets a member's password. Only admins can reset other members' passwords.
func (s *sTenant) ResetMemberPassword(ctx context.Context, req *v1.TenantMemberResetPasswordReq) (*v1.TenantMemberResetPasswordRes, error) {
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)
	currentUserID := middleware.GetUserID(ctx)
	memberID := req.Id

	if memberID == currentUserID {
		return nil, common.NewBadRequestError("不能重置自己的密码，请使用修改密码功能")
	}

	var user *struct {
		Role string `json:"role"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", memberID).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewNotFoundError("成员")
	}
	if user.Role == "owner" {
		return nil, common.NewBadRequestError("不能重置组织所有者的密码")
	}
	if user.Role == "" {
		return nil, common.NewBadRequestError("成员不存在")
	}

	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", memberID).
		Where("tenant_id", tenantID).
		Data(do.TntUsers{
			PasswordHash: passwordHash,
		}).Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// GetMember returns a single member's detail.
func (s *sTenant) GetMember(ctx context.Context, req *v1.TenantMemberGetReq) (*v1.TenantMemberGetRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	var user *struct {
		Id          int64  `json:"id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBadRequestError("成员不存在")
	}

	return &v1.TenantMemberGetRes{
		ID:          user.Id,
		Username:    user.Username,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		Status:      user.Status,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

// GetMemberUsage returns usage statistics for a single member.
func (s *sTenant) GetMemberUsage(ctx context.Context, req *v1.TenantMemberUsageReq) (*v1.TenantMemberUsageRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	// Verify member exists in tenant
	var user *struct {
		Id int64 `json:"id"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBadRequestError("成员不存在")
	}

	now := time.Now()
	today := now.Format("2006-01-02")
	monthStart := now.Format("2006-01") + "-01"

	res := &v1.TenantMemberUsageRes{}

	// Today stats
	todayRecord, err := dao.BilUsageLogs.Ctx(ctx).
		Fields("COALESCE(COUNT(*), 0) as cnt, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Where("tenant_id", tenantID).
		Where("user_id", req.Id).
		Where("created_at >= ?", today+" 00:00:00").
		One()
	if err == nil && todayRecord != nil {
		res.TodayRequests = todayRecord["cnt"].Float64()
	}

	// Month stats
	monthRecord, err := dao.BilUsageLogs.Ctx(ctx).
		Fields("COALESCE(COUNT(*), 0) as cnt, COALESCE(SUM(input_tokens), 0) as input_tokens, COALESCE(SUM(output_tokens), 0) as output_tokens, COALESCE(SUM(total_cost), 0) as total_cost").
		Where("tenant_id", tenantID).
		Where("user_id", req.Id).
		Where("created_at >= ?", monthStart+" 00:00:00").
		One()
	if err == nil && monthRecord != nil {
		res.MonthRequests = monthRecord["cnt"].Float64()
		res.MonthInputTokens = monthRecord["input_tokens"].Float64()
		res.MonthOutputTokens = monthRecord["output_tokens"].Float64()
		res.MonthTotalCost = monthRecord["total_cost"].Float64()
	}

	return res, nil
}

// ListMemberApiKeys returns a paginated list of API keys belonging to a specific member.
func (s *sTenant) ListMemberApiKeys(ctx context.Context, req *v1.TenantMemberApiKeysReq) (*v1.TenantMemberApiKeysRes, error) {
	tenantID := middleware.GetTenantID(ctx)

	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	// Verify member exists in tenant
	var user *struct {
		Id int64 `json:"id"`
	}
	err := dao.TntUsers.Ctx(ctx).
		Where("id", req.Id).
		Where("tenant_id", tenantID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBadRequestError("成员不存在")
	}

	query := dao.ApiKeys.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", req.Id)

	var keys []struct {
		Id         int64   `json:"id"`
		Name       string  `json:"name"`
		KeyPrefix  string  `json:"key_prefix"`
		Scope      string  `json:"scope"`
		Status     string  `json:"status"`
		ExpiresAt  string  `json:"expires_at"`
		CreatedAt  string  `json:"created_at"`
		TotalQuota float64 `json:"total_quota"`
		UsedQuota  float64 `json:"used_quota"`
	}
	var total int
	err = query.OrderDesc("created_at").
		Page(page, pageSize).
		ScanAndCount(&keys, &total, false)
	if err != nil {
		return nil, err
	}

	items := make([]v1.TenantMemberApiKeyItem, len(keys))
	for i, k := range keys {
		items[i] = v1.TenantMemberApiKeyItem{
			ID:         k.Id,
			Name:       k.Name,
			KeyPrefix:  k.KeyPrefix,
			Scope:      k.Scope,
			Status:     k.Status,
			ExpiresAt:  k.ExpiresAt,
			CreatedAt:  k.CreatedAt,
			TotalQuota: k.TotalQuota,
			UsedQuota:  k.UsedQuota,
		}
	}

	return &v1.TenantMemberApiKeysRes{
		List:     items,
		Total:    int(total),
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// ExportMembers exports the tenant member list as CSV or Excel.
func (s *sTenant) ExportMembers(ctx context.Context, req *v1.TenantMemberExportReq) (*v1.TenantMemberExportRes, error) {
	role := middleware.GetUserRole(ctx)
	if role != "owner" && role != "admin" {
		return nil, common.NewForbiddenError("需要 owner 或 admin 权限")
	}
	if err := requireTeamEnabled(ctx); err != nil {
		return nil, err
	}
	tenantID := middleware.GetTenantID(ctx)

	columns := []export.Column{
		{Field: "id", Header: "ID"},
		{Field: "username", Header: "用户名"},
		{Field: "email", Header: "邮箱"},
		{Field: "display_name", Header: "显示名称"},
		{Field: "role", Header: "角色"},
		{Field: "status", Header: "状态"},
		{Field: "created_at", Header: "创建时间"},
	}

	config := export.Config{
		Format:   req.Format,
		Filename: "成员列表_" + gtime.Now().Format("Ymd_His"),
		Columns:  columns,
	}

	return nil, export.GenericExport(ctx, config, func(yield func(map[string]any) bool) {
		offset := 0
		for {
			model := dao.TntUsers.Ctx(ctx).
				Where("tenant_id", tenantID)
			if req.Keyword != "" {
				keyword := "%" + strings.TrimSpace(req.Keyword) + "%"
				model = model.Where("username LIKE ? OR email LIKE ?", keyword, keyword)
			}
			if req.Role != "" {
				model = model.Where("role", req.Role)
			}

			var users []struct {
				Id          int64  `json:"id"`
				Username    string `json:"username"`
				Email       string `json:"email"`
				DisplayName string `json:"display_name"`
				Role        string `json:"role"`
				Status      string `json:"status"`
				CreatedAt   string `json:"created_at"`
			}
			err := model.OrderDesc("id").Limit(1000).Offset(offset).Scan(&users)
			if err = common.IgnoreScanNoRows(err); err != nil {
				return
			}
			for _, u := range users {
				if !yield(map[string]any{
					"id":           u.Id,
					"username":     u.Username,
					"email":        u.Email,
					"display_name": u.DisplayName,
					"role":         u.Role,
					"status":       u.Status,
					"created_at":   u.CreatedAt,
				}) {
					return
				}
			}
			if len(users) < 1000 {
				break
			}
			offset += 1000
		}
	})
}
