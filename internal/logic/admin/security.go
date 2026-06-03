package admin

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// Verify2FA handles the 2FA verification step during admin login.
func (s *sAdmin) Verify2FA(ctx context.Context, req *v1.Admin2FAVerifyReq) (*v1.Admin2FAVerifyRes, error) {
	// Parse provisional token
	claims, err := common.ParseProvisionalToken(req.Provisional)
	if err != nil {
		return nil, common.NewBusinessError(consts.CodeUnauthorized, "临时令牌无效或已过期")
	}

	if claims.UserType != "admin" {
		return nil, common.NewBusinessError(consts.CodeUnauthorized, "令牌类型不匹配")
	}

	// Verify 2FA code
	valid, err := common.Verify2FACode(ctx, "admin", claims.UserID, req.Code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, common.NewBusinessError(consts.CodeTotpInvalid, consts.MsgTotpInvalid)
	}

	// Get user info for response
	var user *entity.SysAdminUsers
	err = dao.SysAdminUsers.Ctx(ctx).Where("id", claims.UserID).Scan(&user)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	if user == nil {
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}
	if user.Status != "active" {
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	// Generate refresh token
	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)

	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
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
	if _, err := dao.SysAdminUsers.Ctx(ctx).Where("id", user.Id).Data(do.SysAdminUsers{
		LastLoginAt: gtime.Now(),
		LastLoginIp: ipAddress,
	}).Update(); err != nil {
		g.Log().Warningf(ctx, "update last_login for user %d: %v", user.Id, err)
	}

	// Record login history
	ua := g.RequestFromCtx(ctx).Header.Get("User-Agent")
	deviceFP := common.DeviceFingerprint(ua, ipAddress)
	_ = common.RecordLoginHistory(ctx, "admin", user.Id, 0, "totp", ipAddress, ua, deviceFP, true, "")

	res := &v1.Admin2FAVerifyRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.User.ID = user.Id
	res.User.Username = user.Username
	res.User.DisplayName = user.DisplayName
	res.User.Role = user.Role
	return res, nil
}

// Setup2FA starts the 2FA setup process for the current admin user.
func (s *sAdmin) Setup2FA(ctx context.Context, _ *v1.Admin2FASetupReq) (*v1.Admin2FASetupRes, error) {
	userID := common.GetCtxUserID(ctx)
	secret, uri, err := common.Setup2FA(ctx, "admin", userID)
	if err != nil {
		return nil, err
	}

	// Store secret in Redis temporarily (5 min TTL) for the enable step
	encKey, err := getEncKey(ctx)
	if err != nil {
		return nil, err
	}
	encrypted, err := crypto.EncryptString(encKey, secret)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("2fa:pending:admin:%d", userID)
	_, err = g.Redis().Do(ctx, "SETEX", key, 300, encrypted)
	if err != nil {
		return nil, err
	}

	return &v1.Admin2FASetupRes{Secret: secret, URI: uri}, nil
}

// Enable2FA confirms and enables 2FA after verifying the code.
func (s *sAdmin) Enable2FA(ctx context.Context, req *v1.Admin2FAEnableReq) (*v1.Admin2FAEnableRes, error) {
	userID := common.GetCtxUserID(ctx)

	// Get pending secret from session/cache (stored temporarily during setup)
	secret, err := getPendingTOTPSecret(ctx, userID)
	if err != nil || secret == "" {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "请先执行2FA设置")
	}

	backupCodes, err := common.Enable2FA(ctx, "admin", userID, secret, req.Code, req.Password)
	if err != nil {
		return nil, err
	}

	// Clear pending secret
	clearPendingTOTPSecret(ctx, userID)

	return &v1.Admin2FAEnableRes{BackupCodes: backupCodes}, nil
}

// Disable2FA disables 2FA for the current admin user.
func (s *sAdmin) Disable2FA(ctx context.Context, req *v1.Admin2FADisableReq) (*v1.Admin2FADisableRes, error) {
	userID := common.GetCtxUserID(ctx)
	err := common.Disable2FA(ctx, "admin", userID, req.Code)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// RegenerateBackupCodes generates new backup codes.
func (s *sAdmin) RegenerateBackupCodes(ctx context.Context, req *v1.Admin2FARegenerateBackupCodesReq) (*v1.Admin2FARegenerateBackupCodesRes, error) {
	userID := common.GetCtxUserID(ctx)
	codes, err := common.RegenerateBackupCodes(ctx, "admin", userID, req.Code)
	if err != nil {
		return nil, err
	}
	return &v1.Admin2FARegenerateBackupCodesRes{BackupCodes: codes}, nil
}

// ConfirmHighRisk generates a confirm token for high-risk operations.
func (s *sAdmin) ConfirmHighRisk(ctx context.Context, req *v1.Admin2FAConfirmReq) (*v1.Admin2FAConfirmRes, error) {
	userID := common.GetCtxUserID(ctx)

	enabled, err := common.Is2FAEnabled(ctx, "admin", userID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, common.NewBusinessError(consts.CodeTotpNotEnabled, consts.MsgTotpNotEnabled)
	}

	valid, err := common.Verify2FACode(ctx, "admin", userID, req.Code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, common.NewBusinessError(consts.CodeTotpInvalid, consts.MsgTotpInvalid)
	}

	token, err := common.GenerateConfirmToken(ctx, userID, "admin")
	if err != nil {
		return nil, err
	}

	return &v1.Admin2FAConfirmRes{ConfirmToken: token}, nil
}

// LoginHistory returns the login history for all admin users with search filters.
func (s *sAdmin) LoginHistory(ctx context.Context, req *v1.AdminLoginHistoryReq) (*v1.AdminLoginHistoryRes, error) {
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	q := common.AuditModelCtx(ctx, "aud_login_history").Where("user_type", "admin")

	if req.Username != "" {
		userIds, err := adminUserIdsByUsername(ctx, req.Username)
		if err != nil {
			return nil, err
		}
		if len(userIds) == 0 {
			return &v1.AdminLoginHistoryRes{List: []v1.LoginHistoryItem{}, Total: 0, Page: page, PageSize: pageSize}, nil
		}
		q = q.WhereIn("user_id", userIds)
	}
	if req.IpAddress != "" {
		q = q.WhereLike("ip_address", "%"+req.IpAddress+"%")
	}
	if req.Success != nil {
		q = q.Where("success", *req.Success)
	}
	if req.LoginMethod != "" {
		q = q.Where("login_method", req.LoginMethod)
	}
	if req.StartTime != "" {
		q = q.WhereGTE("created_at", req.StartTime+" 00:00:00")
	}
	if req.EndTime != "" {
		q = q.WhereLTE("created_at", req.EndTime+" 23:59:59")
	}

	total, err := q.Count()
	if err != nil {
		return nil, err
	}

	var records []entity.AudLoginHistory
	err = q.OrderDesc("created_at").Page(page, pageSize).Scan(&records)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	userMap := buildAdminUserMap(ctx, records)
	items := make([]v1.LoginHistoryItem, len(records))
	for i, r := range records {
		items[i] = v1.LoginHistoryItem{
			ID:          r.Id,
			UserId:      r.UserId,
			Username:    userMap[r.UserId].Username,
			DisplayName: userMap[r.UserId].DisplayName,
			LoginMethod: r.LoginMethod,
			IpAddress:   r.IpAddress,
			UserAgent:   r.UserAgent,
			Location:    r.Location,
			IsNewDevice: r.IsNewDevice,
			Success:     r.Success,
			FailReason:  r.FailReason,
		}
		if r.CreatedAt != nil {
			items[i].CreatedAt = r.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.AdminLoginHistoryRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

type adminUserBrief struct {
	Username    string
	DisplayName string
}

func adminUserIdsByUsername(ctx context.Context, keyword string) ([]int64, error) {
	var users []entity.SysAdminUsers
	err := dao.SysAdminUsers.Ctx(ctx).
		Fields("id").
		WhereLike("username", "%"+keyword+"%").
		WhereOrLike("display_name", "%"+keyword+"%").
		Scan(&users)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.Id
	}
	return ids, nil
}

func buildAdminUserMap(ctx context.Context, records []entity.AudLoginHistory) map[int64]adminUserBrief {
	m := make(map[int64]adminUserBrief)
	idSet := make(map[int64]struct{})
	for _, r := range records {
		idSet[r.UserId] = struct{}{}
	}
	if len(idSet) == 0 {
		return m
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var users []entity.SysAdminUsers
	if err := dao.SysAdminUsers.Ctx(ctx).
		Fields("id, username, display_name").
		WhereIn("id", ids).
		Scan(&users); err != nil {
		g.Log().Warningf(ctx, "load admin users for session list: %v", err)
	}
	for _, u := range users {
		m[u.Id] = adminUserBrief{Username: u.Username, DisplayName: u.DisplayName}
	}
	return m
}

// TenantLoginHistory returns login history for tenant users (admin view).
func (s *sAdmin) TenantLoginHistory(ctx context.Context, req *v1.AdminTenantLoginHistoryReq) (*v1.AdminTenantLoginHistoryRes, error) {
	page, pageSize := common.NormalizePagination(req.Page, req.PageSize)

	q := common.AuditModelCtx(ctx, "aud_login_history").Where("user_type", "tenant")

	if req.TenantID > 0 {
		q = q.Where("tenant_id", req.TenantID)
	}
	if req.Username != "" {
		userIds, err := tenantUserIdsByKeyword(ctx, req.TenantID, req.Username)
		if err != nil {
			return nil, err
		}
		if len(userIds) == 0 {
			return &v1.AdminTenantLoginHistoryRes{
				List: []v1.AdminTenantLoginHistoryItem{}, Total: 0, Page: page, PageSize: pageSize,
			}, nil
		}
		q = q.WhereIn("user_id", userIds)
	}
	if req.IpAddress != "" {
		q = q.WhereLike("ip_address", "%"+req.IpAddress+"%")
	}
	if req.Success != nil {
		q = q.Where("success", *req.Success)
	}
	if req.LoginMethod != "" {
		q = q.Where("login_method", req.LoginMethod)
	}
	if req.StartTime != "" {
		q = q.WhereGTE("created_at", req.StartTime+" 00:00:00")
	}
	if req.EndTime != "" {
		q = q.WhereLTE("created_at", req.EndTime+" 23:59:59")
	}

	total, err := q.Count()
	if err != nil {
		return nil, err
	}

	var records []entity.AudLoginHistory
	err = q.OrderDesc("created_at").Page(page, pageSize).Scan(&records)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}

	userMap := buildTenantUserMap(ctx, records)
	items := make([]v1.AdminTenantLoginHistoryItem, len(records))
	for i, r := range records {
		items[i] = v1.AdminTenantLoginHistoryItem{
			ID:          r.Id,
			UserId:      r.UserId,
			TenantId:    r.TenantId,
			Username:    userMap[r.UserId].Username,
			DisplayName: userMap[r.UserId].DisplayName,
			LoginMethod: r.LoginMethod,
			IpAddress:   r.IpAddress,
			UserAgent:   r.UserAgent,
			Location:    r.Location,
			IsNewDevice: r.IsNewDevice,
			Success:     r.Success,
			FailReason:  r.FailReason,
		}
		if r.CreatedAt != nil {
			items[i].CreatedAt = r.CreatedAt.Format("Y-m-d H:i:s")
		}
	}

	return &v1.AdminTenantLoginHistoryRes{
		List: items, Total: total, Page: page, PageSize: pageSize,
	}, nil
}

type tenantUserBrief struct {
	Username    string
	DisplayName string
}

func tenantUserIdsByKeyword(ctx context.Context, tenantID int64, keyword string) ([]int64, error) {
	q := dao.TntUsers.Ctx(ctx).Fields("id")
	q = q.Where("username LIKE ? OR display_name LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	if tenantID > 0 {
		q = q.Where("tenant_id", tenantID)
	}
	var users []entity.TntUsers
	err := q.Scan(&users)
	if err = common.IgnoreScanNoRows(err); err != nil {
		return nil, err
	}
	ids := make([]int64, len(users))
	for i, u := range users {
		ids[i] = u.Id
	}
	return ids, nil
}

func buildTenantUserMap(ctx context.Context, records []entity.AudLoginHistory) map[int64]tenantUserBrief {
	m := make(map[int64]tenantUserBrief)
	idSet := make(map[int64]struct{})
	for _, r := range records {
		if r.UserId > 0 {
			idSet[r.UserId] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return m
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	var users []entity.TntUsers
	if err := dao.TntUsers.Ctx(ctx).
		Fields("id, username, display_name").
		WhereIn("id", ids).
		Scan(&users); err != nil {
		g.Log().Warningf(ctx, "load tenant users for session list: %v", err)
	}
	for _, u := range users {
		m[u.Id] = tenantUserBrief{Username: u.Username, DisplayName: u.DisplayName}
	}
	return m
}

// Pending TOTP secret is stored in Redis temporarily during setup flow.
func getPendingTOTPSecret(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("2fa:pending:admin:%d", userID)
	val, err := g.Redis().Do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if val.IsNil() || val.IsEmpty() {
		return "", nil
	}
	// The secret is stored encrypted, decrypt it
	encKey, err := getEncKey(ctx)
	if err != nil {
		return "", err
	}
	return crypto.DecryptString(encKey, val.String())
}

func clearPendingTOTPSecret(ctx context.Context, userID int64) {
	key := fmt.Sprintf("2fa:pending:admin:%d", userID)
	_, _ = g.Redis().Do(ctx, "DEL", key)
}

func getEncKey(ctx context.Context) ([]byte, error) {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		return nil, gerror.New("crypto.encryptionKey is not configured")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, gerror.Wrapf(err, "invalid crypto.encryptionKey")
	}
	if len(key) != 32 {
		return nil, gerror.Newf("crypto.encryptionKey must be 32 bytes, got %d", len(key))
	}
	return key, nil
}
