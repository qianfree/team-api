package tenant

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// Register handles tenant registration.
func (s *sTenant) Register(ctx context.Context, req *v1.TenantRegisterReq) (*v1.TenantRegisterRes, error) {
	g.Log().Infof(ctx, "[Register] raw body: %s", g.RequestFromCtx(ctx).GetBodyString())
	g.Log().Infof(ctx, "[Register] parsed req: %+v", req)

	// Check if registration is enabled
	if !common.Config().GetBool(ctx, "register_enabled") {
		return nil, common.NewBusinessError(consts.CodeRegistrationDisabled, consts.MsgRegistrationDisabled)
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	tenantCode := strings.TrimSpace(strings.ToLower(req.TenantCode))
	username := strings.TrimSpace(req.Username)

	// 根据配置选择验证方式
	emailVerificationEnabled := common.Config().GetBool(ctx, "register_email_verification")
	if emailVerificationEnabled {
		// 邮箱验证模式：要求邮箱验证码
		if req.Code == "" {
			return nil, common.NewBusinessError(consts.CodeVerifyCodeInvalid, consts.MsgVerifyCodeInvalid)
		}
		err := common.VerifyCode(ctx, email, req.Code, "register")
		if err != nil {
			return nil, err
		}
	} else {
		// 滑块验证模式
		if err := common.CheckCaptchaRequired(ctx, "tenant_register", req.CaptchaKey, req.CaptchaX); err != nil {
			return nil, err
		}
	}

	// Check tenant code uniqueness
	count, err := dao.TntTenants.Ctx(ctx).
		Where("code", tenantCode).Count()
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, common.NewBusinessError(consts.CodeTenantCodeExists, consts.MsgTenantCodeExists)
	}

	// Validate password policy
	if err := common.ValidatePassword(req.Password); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	passwordHash, err := crypto.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	var tenantID, ownerUserID int64

	ipAddress := g.RequestFromCtx(ctx).GetClientIp()

	err = dao.TntTenants.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Create tenant
		tenantResult, err := tx.Model("tnt_tenants").Ctx(ctx).Data(do.TntTenants{
			Name:       strings.TrimSpace(req.TenantName),
			Code:       tenantCode,
			MaxMembers: 10,
			Settings:   "{}",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create tenant")
		}
		tenantID, err = tenantResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get tenant id")
		}

		// Create owner user
		userResult, err := tx.Model("tnt_users").Ctx(ctx).Data(do.TntUsers{
			TenantId:     tenantID,
			Username:     username,
			Email:        email,
			PasswordHash: passwordHash,
			DisplayName:  username,
			Role:         "owner",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create user")
		}
		ownerUserID, err = userResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get user id")
		}

		// Update tenant owner
		_, err = tx.Model("tnt_tenants").Ctx(ctx).
			Where("id", tenantID).
			Data(do.TntTenants{
				OwnerUserId: ownerUserID,
			}).
			Update()
		if err != nil {
			return gerror.Wrapf(err, "set owner")
		}

		// Create wallet for tenant
		_, err = tx.Model("bil_wallets").Ctx(ctx).Data(do.BilWallets{
			TenantId:      tenantID,
			Balance:       0,
			FrozenBalance: 0,
			Currency:      "CNY",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create wallet")
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

	deviceInfo := extractTenantDeviceInfo(ctx)
	sessionID, err := common.CreateSession(ctx, "tenant", ownerUserID, tenantID, refreshTokenHash, ipAddress, deviceInfo)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, ownerUserID, "tenant", "owner", tenantID, sessionID)
	if err != nil {
		return nil, err
	}

	res := &v1.TenantRegisterRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.Tenant.ID = tenantID
	res.Tenant.Name = strings.TrimSpace(req.TenantName)
	res.Tenant.Code = tenantCode
	res.User.ID = ownerUserID
	res.User.Username = username
	res.User.Role = "owner"

	return res, nil
}

// Login handles tenant user login.
func (s *sTenant) Login(ctx context.Context, req *v1.TenantLoginReq) (*v1.TenantLoginRes, error) {
	account := strings.TrimSpace(req.Account)

	// Check captcha if required
	if err := common.CheckCaptchaRequired(ctx, "tenant_login", req.CaptchaKey, req.CaptchaX); err != nil {
		return nil, err
	}

	var tenant entity.TntTenants
	var user entity.TntUsers

	if req.Type == "admin" {
		// Admin login: account is email, find owner user by email
		email := strings.TrimSpace(strings.ToLower(account))
		err := dao.TntUsers.Ctx(ctx).
			Where("email", email).
			Where("role", "owner").
			Scan(&user)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if user.Id == 0 {
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}

		// Find tenant
		err = dao.TntTenants.Ctx(ctx).
			Where("id", user.TenantId).Scan(&tenant)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if tenant.Id == 0 {
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}
	} else {
		// RAM login: account is username@tenant_code
		parts := strings.SplitN(account, "@", 2)
		if len(parts) != 2 {
			return nil, common.NewBadRequestError("账号格式错误，应为 用户名@组织代码")
		}
		username := parts[0]
		tenantCode := strings.ToLower(parts[1])

		// Find tenant
		err := dao.TntTenants.Ctx(ctx).
			Where("code", tenantCode).Scan(&tenant)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if tenant.Id == 0 {
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}

		// Find user
		err = dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenant.Id).
			Where("username", username).
			Scan(&user)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if user.Id == 0 {
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}
	}

	if tenant.Status != "active" {
		return nil, common.NewBusinessError(consts.CodeTenantSuspended, consts.MsgTenantSuspended)
	}

	// Check account lockout
	if user.LockedUntil != nil && time.Now().Before(user.LockedUntil.Time) {
		remaining := time.Until(user.LockedUntil.Time).Minutes()
		return nil, common.NewBusinessError(consts.CodeAccountLocked,
			fmt.Sprintf("账号已被锁定，%d 分钟后重试", int(remaining)))
	}

	// Check status
	if user.Status != "active" {
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Check IP whitelist
	ipAddr := g.RequestFromCtx(ctx).GetClientIp()
	if !CheckIPWhitelist(ctx, tenant.Id, ipAddr) {
		return nil, common.NewBusinessError(consts.CodeIpRestricted, consts.MsgIpRestricted)
	}

	// Verify password
	if !crypto.VerifyPassword(req.Password, user.PasswordHash) {
		// Increment failed attempts
		failedAttempts := user.FailedAttempts + 1
		updateData := do.TntUsers{FailedAttempts: failedAttempts}

		if failedAttempts >= 5 {
			lockedUntil := time.Now().Add(30 * time.Minute)
			updateData.LockedUntil = gtime.NewFromTime(lockedUntil)
			dao.TntUsers.Ctx(ctx).
				Where("id", user.Id).
				Data(updateData).Update()
			return nil, common.NewBusinessError(consts.CodeAccountLocked,
				"连续 5 次密码错误，账号已锁定 30 分钟")
		}

		dao.TntUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(updateData).Update()

		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Reset failed attempts on successful login
	if user.FailedAttempts > 0 {
		dao.TntUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(do.TntUsers{
				FailedAttempts: 0,
				LockedUntil:    nil,
			}).Update()
	}
	// Get client info
	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	ua := g.RequestFromCtx(ctx).Header.Get("User-Agent")
	deviceFP := common.DeviceFingerprint(ua, ipAddress)

	// Check if 2FA is enabled → return provisional token
	if user.TotpEnabled {
		provisionalToken, err := common.GenerateProvisionalToken(ctx, user.Id, "tenant", user.Role, tenant.Id)
		if err != nil {
			return nil, err
		}

		_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, true, "")

		res := &v1.TenantLoginRes{
			TotpRequired:     true,
			ProvisionalToken: provisionalToken,
		}
		res.Tenant.ID = tenant.Id
		res.Tenant.Name = tenant.Name
		res.Tenant.Code = tenant.Code
		res.User.ID = user.Id
		res.User.Username = user.Username
		res.User.Role = user.Role
		return res, nil
	}

	// Generate tokens
	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)

	// ipAddress already declared above for 2FA check
	deviceInfo := extractTenantDeviceInfo(ctx)
	sessionID, err := common.CreateSession(ctx, "tenant", user.Id, tenant.Id, refreshTokenHash, ipAddress, deviceInfo)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, user.Id, "tenant", user.Role, tenant.Id, sessionID)
	if err != nil {
		return nil, err
	}

	// Update last login
	dao.TntUsers.Ctx(ctx).
		Where("id", user.Id).
		Data(do.TntUsers{
			LastLoginAt: gtime.Now(),
			LastLoginIp: ipAddress,
		}).Update()

	res := &v1.TenantLoginRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.Tenant.ID = tenant.Id
	res.Tenant.Name = tenant.Name
	res.Tenant.Code = tenant.Code
	res.User.ID = user.Id
	res.User.Username = user.Username
	res.User.Role = user.Role

	// Check maintenance mode
	maintenanceMode := common.Config().GetBool(ctx, "maintenance_mode")
	if maintenanceMode {
		res.MaintenanceInfo = &v1.LoginMaintenanceInfo{
			Enabled:  true,
			Message:  common.Config().GetString(ctx, "maintenance_message"),
			Duration: common.Config().GetString(ctx, "maintenance_duration"),
		}
	}

	return res, nil
}

// Logout handles tenant user logout.
func (s *sTenant) Logout(ctx context.Context, req *v1.TenantLogoutReq) (*v1.TenantLogoutRes, error) {
	sessionID := ctxSessionID(ctx)
	common.MarkSessionRevoked(ctx, sessionID)
	err := common.RevokeSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// Refresh handles token refresh for tenant users.
func (s *sTenant) Refresh(ctx context.Context, req *v1.TenantRefreshReq) (*v1.TenantRefreshRes, error) {
	refreshTokenHash := common.HashRefreshToken(req.RefreshToken)

	session, err := common.GetSessionByRefreshHash(ctx, refreshTokenHash)
	if err != nil {
		return nil, common.NewUnauthorizedError("会话不存在")
	}
	if session == nil {
		return nil, common.NewUnauthorizedError("会话已过期或不存在")
	}
	if session.UserType != "tenant" {
		return nil, common.NewUnauthorizedError("令牌类型不匹配")
	}
	if common.IsSessionRevoked(ctx, session.Id) {
		return nil, common.NewBusinessError(consts.CodeTokenRevoked, consts.MsgTokenRevoked)
	}

	newRefreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	newRefreshTokenHash := common.HashRefreshToken(newRefreshToken)

	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	deviceInfo := extractTenantDeviceInfo(ctx)
	err = common.RefreshSession(ctx, session.Id, refreshTokenHash, newRefreshTokenHash, ipAddress, deviceInfo)
	if err != nil {
		return nil, err
	}

	// Fetch current role from user table
	var tntUser entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).Where("id", session.UserId).Fields("role").Scan(&tntUser)
	if err != nil {
		return nil, err
	}

	tokenPair, err := common.GenerateTokenPair(ctx, session.UserId, "tenant", tntUser.Role, session.TenantId, session.Id)
	if err != nil {
		return nil, err
	}

	return &v1.TenantRefreshRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// ChangePassword handles tenant user password change.
func (s *sTenant) ChangePassword(ctx context.Context, req *v1.TenantChangePasswordReq) (*v1.TenantChangePasswordRes, error) {
	userID := ctxUserID(ctx)

	var user entity.TntUsers
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).Scan(&user)
	if err != nil {
		return nil, err
	}

	if !crypto.VerifyPassword(req.OldPassword, user.PasswordHash) {
		return nil, common.NewBusinessError(consts.CodeOldPasswordWrong, consts.MsgOldPasswordWrong)
	}

	if err := common.ValidatePassword(req.NewPassword); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, err
	}

	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", userID).
		Data(do.TntUsers{
			PasswordHash: newHash,
		}).Update()
	if err != nil {
		return nil, err
	}

	common.RevokeAllSessions(ctx, "tenant", userID)
	return nil, nil
}

// ListSessions returns active sessions for the current tenant user.
func (s *sTenant) ListSessions(ctx context.Context, req *v1.TenantSessionListReq) (*v1.TenantSessionListRes, error) {
	userID := ctxUserID(ctx)
	currentSessionID := ctxSessionID(ctx)

	sessions, err := common.ListSessions(ctx, "tenant", userID)
	if err != nil {
		return nil, err
	}

	items := make([]v1.TenantSessionItem, len(sessions))
	for i, sess := range sessions {
		items[i] = v1.TenantSessionItem{
			ID:         sess.ID,
			IpAddress:  sess.IpAddress,
			DeviceInfo: sess.DeviceInfo,
			IsCurrent:  sess.ID == currentSessionID,
		}
		if sess.ExpiresAt != nil {
			items[i].ExpiresAt = sess.ExpiresAt.Format("Y-m-d H:i:s")
		}
		if sess.CreatedAt != nil {
			items[i].CreatedAt = sess.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.TenantSessionListRes{List: items}, nil
}

// RevokeSession revokes a specific session.
func (s *sTenant) RevokeSession(ctx context.Context, req *v1.TenantRevokeSessionReq) (*v1.TenantRevokeSessionRes, error) {
	common.MarkSessionRevoked(ctx, req.Id)
	err := common.RevokeSession(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// extractTenantDeviceInfo extracts device information from the request and returns it as a JSON string.
// The sys_sessions.device_info column is JSONB, so this must be valid JSON.
func extractTenantDeviceInfo(ctx context.Context) string {
	r := g.RequestFromCtx(ctx)
	if r == nil {
		return `{"user_agent":"unknown"}`
	}
	ua := r.Header.Get("User-Agent")
	if len(ua) > 500 {
		ua = ua[:500]
	}
	if ua == "" {
		ua = "unknown"
	}
	b, _ := json.Marshal(map[string]string{"user_agent": ua})
	return string(b)
}
