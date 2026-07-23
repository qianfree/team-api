package admin

import (
	"context"
	"encoding/json"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// NOTE: ctxUserID and ctxSessionID have been removed.
// Use common.GetCtxUserID(ctx) and common.GetCtxSessionID(ctx) instead.

// Login handles admin login.
func (s *sAdmin) Login(ctx context.Context, req *v1.AdminLoginReq) (*v1.AdminLoginRes, error) {
	req.Username = strings.TrimSpace(req.Username)

	// Check captcha if required
	if err := common.CheckCaptchaRequired(ctx, "admin_login", req.CaptchaKey, req.CaptchaX); err != nil {
		return nil, err
	}

	// Get client info early for login history recording
	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	ua := g.RequestFromCtx(ctx).Header.Get("User-Agent")
	deviceFP := common.DeviceFingerprint(ua, ipAddress)

	// Find admin user by username
	var user *entity.SysAdminUsers
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("username", req.Username).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		_ = common.RecordLoginHistory(ctx, "admin", 0, 0, "password", ipAddress, ua, deviceFP, false, "用户不存在")
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Check status
	if user.Status != "active" {
		_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "password", ipAddress, ua, deviceFP, false, "账号已禁用")
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Check account lockout
	if user.LockedUntil != nil {
		if time.Now().Before(user.LockedUntil.Time) {
			// 仍在锁定期内
			remaining := time.Until(user.LockedUntil.Time).Minutes()
			_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "password", ipAddress, ua, deviceFP, false, "账号已锁定")
			return nil, common.NewBusinessError(consts.CodeAccountLocked,
				fmt.Sprintf("账号已被锁定，%d 分钟后重试", int(remaining)))
		}
		// 锁定已过期：重置失败计数，给用户重新 5 次机会（方案A）
		_, err := dao.SysAdminUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(map[string]interface{}{
				"failed_attempts": 0,
				"locked_until":    nil,
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "重置管理员锁定状态失败: %v", err)
		}
		// 同步更新内存对象，避免后续密码校验仍读到旧的失败计数
		user.FailedAttempts = 0
		user.LockedUntil = nil
	}

	// Verify password
	if !crypto.VerifyPassword(req.Password, user.PasswordHash) {
		_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "password", ipAddress, ua, deviceFP, false, "密码错误")

		// 原子递增失败次数
		updateData := do.SysAdminUsers{FailedAttempts: gdb.Raw("failed_attempts + 1")}
		failedAttempts := user.FailedAttempts + 1

		if failedAttempts >= 5 {
			lockedUntil := time.Now().Add(30 * time.Minute)
			updateData.LockedUntil = gtime.NewFromTime(lockedUntil)
			_, err := dao.SysAdminUsers.Ctx(ctx).
				Where("id", user.Id).
				Data(updateData).Update()
			if err != nil {
				g.Log().Errorf(ctx, "更新管理员账号锁定状态失败: %v", err)
			}
			return nil, common.NewBusinessError(consts.CodeAccountLocked,
				"连续 5 次密码错误，账号已锁定 30 分钟")
		}

		_, err := dao.SysAdminUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(updateData).Update()
		if err != nil {
			g.Log().Errorf(ctx, "更新管理员登录失败次数失败: %v", err)
		}

		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Reset failed attempts on successful login
	if user.FailedAttempts > 0 {
		_, err := dao.SysAdminUsers.Ctx(ctx).
			Where("id", user.Id).
			Data(map[string]interface{}{
				"failed_attempts": 0,
				"locked_until":    nil,
			}).Update()
		if err != nil {
			g.Log().Errorf(ctx, "重置管理员登录失败次数失败: %v", err)
		}
	}

	// Check if 2FA is enabled
	if user.TotpEnabled {
		provisionalToken, err := common.GenerateProvisionalToken(ctx, user.Id, "admin", user.Role, 0)
		if err != nil {
			return nil, err
		}

		// Record login attempt (password passed, awaiting 2FA)
		_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "password", ipAddress, ua, deviceFP, true, "")

		res := &v1.AdminLoginRes{
			TotpRequired:     true,
			ProvisionalToken: provisionalToken,
		}
		res.User.ID = user.Id
		res.User.Username = user.Username
		res.User.DisplayName = user.DisplayName
		res.User.Role = user.Role
		return res, nil
	}

	// Generate refresh token
	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)

	deviceInfo := extractDeviceInfo(ctx)

	// Create session with jti
	jti := common.GenerateJti()
	sessionID, err := common.CreateSession(ctx, "admin", user.Id, 0, refreshTokenHash, ipAddress, deviceInfo, jti)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	// Generate token pair
	tokenPair, err := common.GenerateTokenPair(ctx, user.Id, "admin", user.Role, 0, sessionID, jti)
	if err != nil {
		return nil, err
	}

	// Update last login (non-critical, log errors only)
	if _, err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", user.Id).
		Data(do.SysAdminUsers{
			LastLoginAt: gtime.Now(),
			LastLoginIp: ipAddress,
		}).Update(); err != nil {
		g.Log().Warningf(ctx, "update last_login for user %d: %v", user.Id, err)
	}

	// Record login history
	_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "password", ipAddress, ua, deviceFP, true, "")

	// Publish login event
	common.Publish(ctx, &common.Event{
		Type:     "admin.login",
		TenantID: 0,
		UserID:   user.Id,
		Payload: map[string]any{
			"username":   user.Username,
			"ip_address": ipAddress,
		},
	})

	res := &v1.AdminLoginRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.User.ID = user.Id
	res.User.Username = user.Username
	res.User.DisplayName = user.DisplayName
	res.User.Role = user.Role

	// 检查待接受协议
	if common.Config().GetBool(ctx, "agreement_enabled") {
		if pending, err := common.GetPendingAgreements(ctx, "admin", user.Id); err == nil && len(pending) > 0 {
			items := make([]*v1.LoginPendingAgreement, 0, len(pending))
			for _, a := range pending {
				items = append(items, &v1.LoginPendingAgreement{
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

// Logout handles admin logout.
func (s *sAdmin) Logout(ctx context.Context, _ *v1.AdminLogoutReq) (*v1.AdminLogoutRes, error) {
	jti := common.GetCtxJti(ctx)
	sessionID := common.GetCtxSessionID(ctx)

	// Mark session as revoked in Redis for instant effect
	common.MarkSessionRevoked(ctx, jti)

	// Delete session from database
	err := common.RevokeSession(ctx, "admin", sessionID)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// Refresh handles token refresh.
func (s *sAdmin) Refresh(ctx context.Context, req *v1.AdminRefreshReq) (*v1.AdminRefreshRes, error) {
	refreshTokenHash := common.HashRefreshToken(req.RefreshToken)

	// Find session by refresh token hash
	session, err := common.GetSessionByRefreshHash(ctx, refreshTokenHash)
	if err != nil {
		return nil, common.NewUnauthorizedError("会话不存在")
	}
	if session == nil {
		return nil, common.NewUnauthorizedError("会话已过期或不存在")
	}

	if session.UserType != "admin" {
		return nil, common.NewUnauthorizedError("令牌类型不匹配")
	}

	// Check if session is revoked
	if common.IsSessionRevoked(ctx, session.Jti) {
		return nil, common.NewBusinessError(consts.CodeTokenRevoked, consts.MsgTokenRevoked)
	}

	// Generate new refresh token
	newRefreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	newRefreshTokenHash := common.HashRefreshToken(newRefreshToken)

	// Rotate session
	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	deviceInfo := extractDeviceInfo(ctx)
	err = common.RefreshSession(ctx, session.Id, refreshTokenHash, newRefreshTokenHash, ipAddress, deviceInfo)
	if err != nil {
		return nil, err
	}

	// Fetch current role from user table
	var adminUser *entity.SysAdminUsers
	err = dao.SysAdminUsers.Ctx(ctx).Where("id", session.UserId).Fields("role, status").Scan(&adminUser)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if adminUser == nil || adminUser.Status != "active" {
		return nil, common.NewUnauthorizedError("账号已被禁用或不存在")
	}

	// Generate new token pair with same session ID and jti
	tokenPair, err := common.GenerateTokenPair(ctx, session.UserId, session.UserType, adminUser.Role, 0, session.Id, session.Jti)
	if err != nil {
		return nil, err
	}

	return &v1.AdminRefreshRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: newRefreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// ListSessions returns active sessions for the current admin user.
func (s *sAdmin) ListSessions(ctx context.Context, req *v1.AdminSessionListReq) (*v1.AdminSessionListRes, error) {
	currentSessionID := common.GetCtxSessionID(ctx)
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := dao.SysSessions.Ctx(ctx).
		Where("user_type", "admin").
		Where("expires_at > NOW()")

	if req.Username != "" {
		userIds, err := adminUserIdsByUsername(ctx, req.Username)
		if err != nil {
			return nil, err
		}
		if len(userIds) == 0 {
			return &v1.AdminSessionListRes{List: []v1.AdminSessionItem{}, Total: 0, Page: page, PageSize: pageSize}, nil
		}
		q = q.WhereIn("user_id", userIds)
	}
	if req.IpAddress != "" {
		q = q.WhereLike("ip_address", "%"+req.IpAddress+"%")
	}

	total, err := q.Count()
	if err != nil {
		return nil, err
	}

	var sessions []entity.SysSessions
	err = q.OrderDesc("created_at").Page(page, pageSize).Scan(&sessions)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	idSet := make(map[int64]struct{})
	for _, s := range sessions {
		idSet[s.UserId] = struct{}{}
	}
	userMap := make(map[int64]adminUserBrief)
	if len(idSet) > 0 {
		ids := make([]int64, 0, len(idSet))
		for id := range idSet {
			ids = append(ids, id)
		}
		var users []entity.SysAdminUsers
		_ = dao.SysAdminUsers.Ctx(ctx).Fields("id, username, display_name").WhereIn("id", ids).Scan(&users)
		for _, u := range users {
			userMap[u.Id] = adminUserBrief{Username: u.Username, DisplayName: u.DisplayName}
		}
	}

	items := make([]v1.AdminSessionItem, len(sessions))
	for i, sess := range sessions {
		items[i] = v1.AdminSessionItem{
			ID:          sess.Id,
			UserId:      sess.UserId,
			Username:    userMap[sess.UserId].Username,
			DisplayName: userMap[sess.UserId].DisplayName,
			IpAddress:   sess.IpAddress,
			DeviceInfo:  sess.DeviceInfo,
			IsCurrent:   sess.Id == currentSessionID,
		}
		if sess.ExpiresAt != nil {
			items[i].ExpiresAt = sess.ExpiresAt.Format("Y-m-d H:i:s")
		}
		if sess.CreatedAt != nil {
			items[i].CreatedAt = sess.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.AdminSessionListRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// RevokeSession revokes a specific session.
func (s *sAdmin) RevokeSession(ctx context.Context, req *v1.AdminRevokeSessionReq) (*v1.AdminRevokeSessionRes, error) {
	// Look up session to get jti for Redis revocation
	sess, err := common.GetSessionByID(ctx, "admin", req.Id)
	if err != nil {
		return nil, err
	}
	if sess != nil {
		common.MarkSessionRevoked(ctx, sess.Jti)
	}
	err = common.RevokeSession(ctx, "admin", req.Id)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ForceLogout revokes all sessions for a specific user.
func (s *sAdmin) ForceLogout(ctx context.Context, req *v1.AdminForceLogoutReq) (*v1.AdminForceLogoutRes, error) {
	err := common.RevokeAllSessions(ctx, "admin", req.Id)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// ChangePassword handles admin password change.
func (s *sAdmin) ChangePassword(ctx context.Context, req *v1.AdminChangePasswordReq) (*v1.AdminChangePasswordRes, error) {
	userID := common.GetCtxUserID(ctx)

	var user *entity.SysAdminUsers
	err := dao.SysAdminUsers.Ctx(ctx).
		Where("id", userID).
		Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewUnauthorizedError("账号已被禁用或不存在")
	}

	// Verify old password
	if !crypto.VerifyPassword(req.OldPassword, user.PasswordHash) {
		return nil, common.NewBusinessError(consts.CodeOldPasswordWrong, consts.MsgOldPasswordWrong)
	}

	// Validate new password policy
	if err := common.ValidatePassword(req.NewPassword); err != nil {
		return nil, common.NewBusinessError(consts.CodePasswordTooWeak, consts.MsgPasswordTooWeak)
	}

	// Hash new password
	newHash, err := crypto.HashPassword(req.NewPassword)
	if err != nil {
		return nil, err
	}

	// Update password
	_, err = dao.SysAdminUsers.Ctx(ctx).
		Where("id", userID).
		Data(do.SysAdminUsers{
			PasswordHash: newHash,
		}).Update()
	if err != nil {
		return nil, err
	}

	// Revoke all sessions (force re-login on all devices)
	common.RevokeAllSessions(ctx, "admin", userID)

	return nil, nil
}

// extractDeviceInfo extracts device information from the request and returns it as a JSON string.
// The sys_sessions.device_info column is JSONB, so this must be valid JSON.
func extractDeviceInfo(ctx context.Context) string {
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
