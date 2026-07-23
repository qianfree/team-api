package tenant

import (
	"context"
	"encoding/json"
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
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/middleware"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"

	do "github.com/qianfree/team-api/internal/model/do"
)

// Register handles tenant registration.
func (s *sTenant) Register(ctx context.Context, req *v1.TenantRegisterReq) (*v1.TenantRegisterRes, error) {
	// Check if registration is enabled
	if !common.Config().GetBool(ctx, "register_enabled") {
		return nil, common.NewBusinessError(consts.CodeRegistrationDisabled, consts.MsgRegistrationDisabled)
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	tenantCode := strings.TrimSpace(strings.ToLower(req.TenantCode))
	username := strings.TrimSpace(req.Username)
	tenantName := strings.TrimSpace(req.TenantName)

	// 简化注册：组织信息可选，留空时后端自动生成（个人模式，team_enabled=false）
	userProvidedCode := tenantCode != ""
	userProvidedName := tenantName != ""

	// 先校验用户名禁用词：命中禁用词时优先返回更准确的提示，
	// 避免被用户名格式校验的「不能包含特殊字符或中文」遮盖真实原因
	if err := common.ValidateForbiddenWords(ctx, username, "用户名"); err != nil {
		return nil, common.NewBusinessError(consts.CodeForbiddenWord, err.Error())
	}

	// 组织代码：留空则自动生成（org-<base36>）；用户显式传入才校验禁用词
	if !userProvidedCode {
		generated, err := generateUniqueTenantCode(ctx)
		if err != nil {
			return nil, err
		}
		tenantCode = generated
	} else {
		if err := common.ValidateForbiddenWords(ctx, tenantCode, "组织代码"); err != nil {
			return nil, common.NewBusinessError(consts.CodeForbiddenWord, err.Error())
		}
	}

	// 组织名称：留空则用用户名派生默认值；用户显式传入才校验禁用词
	if !userProvidedName {
		// 用户名较长时“xxx 的组织”可能超出组织名称显示宽度上限，按宽度截断兜底
		tenantName = common.TruncateToDisplayWidth(fmt.Sprintf("%s 的组织", username), common.TenantNameMaxDisplayWidth)
	} else {
		if err := common.ValidateTenantName(tenantName); err != nil {
			return nil, common.NewBusinessError(consts.CodeInvalidTenantName, err.Error())
		}
		if err := common.ValidateForbiddenWords(ctx, tenantName, "组织名称"); err != nil {
			return nil, common.NewBusinessError(consts.CodeForbiddenWord, err.Error())
		}
	}

	// Validate username format
	if err := common.ValidateUsername(username); err != nil {
		return nil, common.NewBusinessError(consts.CodeInvalidUsername, err.Error())
	}

	// Check register rate limit (IP + global)
	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	if err := common.CheckRegisterRateLimit(ctx, ipAddress); err != nil {
		return nil, err
	}

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
		// 人机验证：Turnstile 启用时只验 Turnstile，否则用滑块验证
		turnstileEnabled := common.Config().GetBool(ctx, "turnstile_enabled")
		if turnstileEnabled {
			if err := common.CheckTurnstileRequired(ctx, req.TurnstileToken); err != nil {
				return nil, err
			}
		} else {
			if err := common.CheckCaptchaRequired(ctx, "tenant_register", req.CaptchaKey, req.CaptchaX); err != nil {
				return nil, err
			}
		}
	}

	// Check tenant code uniqueness（仅用户显式传入时；自动生成的已在生成时查重）
	if userProvidedCode {
		count, err := dao.TntTenants.Ctx(ctx).
			Where("code", tenantCode).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, common.NewBusinessError(consts.CodeTenantCodeExists, consts.MsgTenantCodeExists)
		}
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

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Create tenant
		tenantResult, err := dao.TntTenants.Ctx(ctx).Data(do.TntTenants{
			Name: tenantName,
			Code: tenantCode,
			// MaxMembers/MaxConcurrency: nil = 跟随等级配置
			// Level 1 defaults will apply
			Level:       1,
			Settings:    "{}",
			TeamEnabled: false, // 新注册默认个人模式，设置自定义 code 后才激活团队功能
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create tenant")
		}
		tenantID, err = tenantResult.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get tenant id")
		}

		// Create owner user
		userResult, err := dao.TntUsers.Ctx(ctx).Data(do.TntUsers{
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
		_, err = dao.TntTenants.Ctx(ctx).
			Where("id", tenantID).
			Data(do.TntTenants{
				OwnerUserId: ownerUserID,
			}).
			Update()
		if err != nil {
			return gerror.Wrapf(err, "set owner")
		}

		// Create wallet for tenant
		_, err = dao.BilWallets.Ctx(ctx).Data(do.BilWallets{
			TenantId:      tenantID,
			Balance:       0,
			FrozenBalance: 0,
			Currency:      "USD",
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "create wallet")
		}

		// Assign default model groups
		var defaultGroups []struct {
			Id int64
		}
		if err := dao.MdlModelGroups.Ctx(ctx).
			Where("is_default", true).Where("status", "active").
			Fields("id").Scan(&defaultGroups); err == nil && len(defaultGroups) > 0 {
			insertData := make([]do.MdlTenantGroups, 0, len(defaultGroups))
			for _, dg := range defaultGroups {
				insertData = append(insertData, do.MdlTenantGroups{
					TenantId: tenantID,
					GroupId:  dg.Id,
				})
			}
			if _, err := dao.MdlTenantGroups.Ctx(ctx).
				Batch(len(insertData)).Insert(insertData); err != nil {
				g.Log().Warningf(ctx, "assign default model groups failed: %v", err)
			}
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
	jti := common.GenerateJti()
	sessionID, err := common.CreateSession(ctx, "tenant", ownerUserID, tenantID, refreshTokenHash, ipAddress, deviceInfo, jti)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, ownerUserID, "tenant", "owner", tenantID, sessionID, jti)
	if err != nil {
		return nil, err
	}

	res := &v1.TenantRegisterRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.Tenant.ID = tenantID
	res.Tenant.Name = tenantName
	res.Tenant.Code = tenantCode
	res.Tenant.TeamEnabled = false
	res.User.ID = ownerUserID
	res.User.Username = username
	res.User.Role = "owner"

	// 检查待接受协议
	if common.Config().GetBool(ctx, "agreement_enabled") {
		if pending, err := common.GetPendingAgreements(ctx, "tenant", ownerUserID); err == nil && len(pending) > 0 {
			items := make([]*v1.TenantLoginPendingAgreement, 0, len(pending))
			for _, a := range pending {
				items = append(items, &v1.TenantLoginPendingAgreement{
					Id:      a.Id,
					Code:    a.Code,
					Title:   a.Title,
					Version: a.Version,
				})
			}
			res.PendingAgreements = items
		}
	}

	return res, nil
}

// Login handles tenant user login.
func (s *sTenant) Login(ctx context.Context, req *v1.TenantLoginReq) (*v1.TenantLoginRes, error) {
	account := strings.TrimSpace(req.Account)

	// Check captcha: Turnstile takes priority; slider captcha is fallback
	turnstileEnabled := common.Config().GetBool(ctx, "turnstile_enabled")
	if turnstileEnabled {
		if err := common.CheckTurnstileRequired(ctx, req.TurnstileToken); err != nil {
			return nil, err
		}
	} else {
		if err := common.CheckCaptchaRequired(ctx, "tenant_login", req.CaptchaKey, req.CaptchaX); err != nil {
			return nil, err
		}
	}

	// Get client info early for login history recording
	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	ua := g.RequestFromCtx(ctx).Header.Get("User-Agent")
	deviceFP := common.DeviceFingerprint(ua, ipAddress)

	var tenant *entity.TntTenants
	var user *entity.TntUsers

	if req.Type == "admin" {
		// Admin login: account is email, find owner user by email
		email := strings.TrimSpace(strings.ToLower(account))
		err := dao.TntUsers.Ctx(ctx).
			Where("email", email).
			Where("role", "owner").
			Scan(&user)
		if err != nil {
			return nil, err
		}
		if user == nil {
			_ = common.RecordLoginHistory(ctx, "tenant", 0, 0, "password", ipAddress, ua, deviceFP, false, "用户不存在")
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}

		// Find tenant
		err = dao.TntTenants.Ctx(ctx).
			Where("id", user.TenantId).Scan(&tenant)
		if err != nil {
			return nil, err
		}
		if tenant == nil {
			_ = common.RecordLoginHistory(ctx, "tenant", user.Id, user.TenantId, "password", ipAddress, ua, deviceFP, false, "租户不存在")
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}
	} else {
		// RAM login: account is username@tenant_code
		parts := strings.SplitN(account, "@", 2)
		if len(parts) != 2 {
			_ = common.RecordLoginHistory(ctx, "tenant", 0, 0, "password", ipAddress, ua, deviceFP, false, "账号格式错误")
			return nil, common.NewBadRequestError("账号格式错误，应为 用户名@组织代码")
		}
		username := parts[0]
		tenantCode := strings.ToLower(parts[1])

		// Find tenant
		err := dao.TntTenants.Ctx(ctx).
			Where("code", tenantCode).Scan(&tenant)
		if err != nil {
			return nil, err
		}
		if tenant == nil {
			_ = common.RecordLoginHistory(ctx, "tenant", 0, 0, "password", ipAddress, ua, deviceFP, false, "租户不存在")
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}

		// Find user
		err = dao.TntUsers.Ctx(ctx).
			Where("tenant_id", tenant.Id).
			Where("username", username).
			Scan(&user)
		if err != nil {
			return nil, err
		}
		if user == nil {
			_ = common.RecordLoginHistory(ctx, "tenant", 0, tenant.Id, "password", ipAddress, ua, deviceFP, false, "用户不存在")
			return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
		}
	}

	if tenant.Status != "active" {
		_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, false, "租户已停用")
		return nil, common.NewBusinessError(consts.CodeTenantSuspended, consts.MsgTenantSuspended)
	}

	// Check account lockout
	if user.LockedUntil != nil {
		if time.Now().Before(user.LockedUntil.Time) {
			// 仍在锁定期内
			remaining := time.Until(user.LockedUntil.Time).Minutes()
			_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, false, "账号已锁定")
			return nil, common.NewBusinessError(consts.CodeAccountLocked,
				fmt.Sprintf("账号已被锁定，%d 分钟后重试", int(remaining)))
		}
		// 锁定已过期：重置失败计数，给用户重新 5 次机会（方案A）
		_, err := dao.TntUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(map[string]interface{}{
				"failed_attempts": 0,
				"locked_until":    nil,
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "重置租户账号锁定状态失败: %v", err)
		}
		// 同步更新内存对象，避免后续密码校验仍读到旧的失败计数
		user.FailedAttempts = 0
		user.LockedUntil = nil
	}

	// Check status
	if user.Status != "active" {
		_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, false, "账号已禁用")
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Check IP whitelist
	if !CheckIPWhitelist(ctx, tenant.Id, ipAddress) {
		_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, false, "IP不在白名单")
		return nil, common.NewBusinessError(consts.CodeIpRestricted, consts.MsgIpRestricted)
	}

	// Verify password
	if !crypto.VerifyPassword(req.Password, user.PasswordHash) {
		_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, false, "密码错误")

		// 原子递增失败次数
		updateData := do.TntUsers{FailedAttempts: gdb.Raw("failed_attempts + 1")}
		failedAttempts := user.FailedAttempts + 1

		if failedAttempts >= 5 {
			lockedUntil := time.Now().Add(30 * time.Minute)
			updateData.LockedUntil = gtime.NewFromTime(lockedUntil)
			_, err := dao.TntUsers.Ctx(ctx).
				Where("id", user.Id).
				Data(updateData).Update()
			if err != nil {
				g.Log().Errorf(ctx, "更新账号锁定状态失败: %v", err)
			}
			return nil, common.NewBusinessError(consts.CodeAccountLocked,
				"连续 5 次密码错误，账号已锁定 30 分钟")
		}

		_, err := dao.TntUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(updateData).Update()
		if err != nil {
			g.Log().Errorf(ctx, "更新登录失败次数失败: %v", err)
		}

		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Reset failed attempts on successful login
	if user.FailedAttempts > 0 {
		_, err := dao.TntUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(map[string]interface{}{
				"failed_attempts": 0,
				"locked_until":    nil,
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "重置登录失败次数失败: %v", err)
		}
	}

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
		res.Tenant.TeamEnabled = tenant.TeamEnabled
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
	jti := common.GenerateJti()
	sessionID, err := common.CreateSession(ctx, "tenant", user.Id, tenant.Id, refreshTokenHash, ipAddress, deviceInfo, jti)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, user.Id, "tenant", user.Role, tenant.Id, sessionID, jti)
	if err != nil {
		return nil, err
	}

	// Update last login
	_, err = dao.TntUsers.Ctx(ctx).
		Where("id", user.Id).
		Data(do.TntUsers{
			LastLoginAt: gtime.Now(),
			LastLoginIp: ipAddress,
		}).Update()
	if err != nil {
		g.Log().Errorf(ctx, "更新最后登录时间失败: %v", err)
	}

	_ = common.RecordLoginHistory(ctx, "tenant", user.Id, tenant.Id, "password", ipAddress, ua, deviceFP, true, "")

	res := &v1.TenantLoginRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.Tenant.ID = tenant.Id
	res.Tenant.Name = tenant.Name
	res.Tenant.Code = tenant.Code
	res.Tenant.TeamEnabled = tenant.TeamEnabled
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

	// 检查待接受协议
	if common.Config().GetBool(ctx, "agreement_enabled") {
		if pending, err := common.GetPendingAgreements(ctx, "tenant", user.Id); err == nil && len(pending) > 0 {
			items := make([]*v1.TenantLoginPendingAgreement, 0, len(pending))
			for _, a := range pending {
				items = append(items, &v1.TenantLoginPendingAgreement{
					Id:      a.Id,
					Code:    a.Code,
					Title:   a.Title,
					Version: a.Version,
				})
			}
			res.PendingAgreements = items
		}
	}

	return res, nil
}

// Logout handles tenant user logout.
func (s *sTenant) Logout(ctx context.Context, req *v1.TenantLogoutReq) (*v1.TenantLogoutRes, error) {
	jti := middleware.GetJti(ctx)
	sessionID := middleware.GetSessionID(ctx)
	common.MarkSessionRevoked(ctx, jti)
	err := common.RevokeSession(ctx, "tenant", sessionID)
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
	if common.IsSessionRevoked(ctx, session.Jti) {
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
	var tntUser *entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).Where("id", session.UserId).Fields("role").Scan(&tntUser)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if tntUser == nil {
		return nil, common.NewUnauthorizedError("用户不存在")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, session.UserId, "tenant", tntUser.Role, session.TenantId, session.Id, session.Jti)
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
	userID := middleware.GetUserID(ctx)

	var user *entity.TntUsers
	err := dao.TntUsers.Ctx(ctx).
		Where("id", userID).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
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
	userID := middleware.GetUserID(ctx)
	currentSessionID := middleware.GetSessionID(ctx)

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

// RevokeSession revokes a specific session (only own sessions).
func (s *sTenant) RevokeSession(ctx context.Context, req *v1.TenantRevokeSessionReq) (*v1.TenantRevokeSessionRes, error) {
	userID := middleware.GetUserID(ctx)

	// 查找 session 并验证归属
	sess, err := common.GetSessionByID(ctx, "tenant", req.Id)
	if err != nil {
		return nil, err
	}
	if sess == nil {
		return nil, common.NewNotFoundError("会话")
	}
	if sess.UserId != userID {
		return nil, common.NewForbiddenError("只能撤销自己的会话")
	}

	common.MarkSessionRevoked(ctx, sess.Jti)
	err = common.RevokeSession(ctx, "tenant", req.Id)
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
