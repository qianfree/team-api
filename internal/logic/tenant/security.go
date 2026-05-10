package tenant

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	v1 "github.com/qianfree/team-api/api/tenant/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// Verify2FA handles the 2FA verification step during tenant login.
func (s *sTenant) Verify2FA(ctx context.Context, req *v1.Tenant2FAVerifyReq) (*v1.Tenant2FAVerifyRes, error) {
	claims, err := common.ParseProvisionalToken(req.Provisional)
	if err != nil {
		return nil, common.NewBusinessError(consts.CodeUnauthorized, "临时令牌无效或已过期")
	}

	if claims.UserType != "tenant" {
		return nil, common.NewBusinessError(consts.CodeUnauthorized, "令牌类型不匹配")
	}

	valid, err := common.Verify2FACode(ctx, "tenant", claims.UserID, req.Code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, common.NewBusinessError(consts.CodeTotpInvalid, consts.MsgTotpInvalid)
	}

	var user entity.TntUsers
	err = dao.TntUsers.Ctx(ctx).Where("id", claims.UserID).Scan(&user)
	if err != nil {
		return nil, err
	}
	if user.Status != "active" {
		return nil, common.NewBusinessError(consts.CodeInvalidCredentials, consts.MsgInvalidCredentials)
	}

	refreshToken, err := common.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	refreshTokenHash := common.HashRefreshToken(refreshToken)

	ipAddress := g.RequestFromCtx(ctx).GetClientIp()
	deviceInfo := extractTenantDeviceInfo(ctx)

	sessionID, err := common.CreateSession(ctx, "tenant", user.Id, user.TenantId, refreshTokenHash, ipAddress, deviceInfo)
	if err != nil {
		return nil, gerror.Wrapf(err, "create session")
	}

	tokenPair, err := common.GenerateTokenPair(ctx, user.Id, "tenant", user.Role, user.TenantId, sessionID)
	if err != nil {
		return nil, err
	}

	dao.TntUsers.Ctx(ctx).Where("id", user.Id).Data(do.TntUsers{
		LastLoginAt: gtime.Now(),
		LastLoginIp: ipAddress,
	}).Update()

	ua := g.RequestFromCtx(ctx).Header.Get("User-Agent")
	deviceFP := common.DeviceFingerprint(ua, ipAddress)
	_ = common.RecordLoginHistory(ctx, "tenant", user.Id, user.TenantId, "totp", ipAddress, ua, deviceFP, true, "")

	// Get tenant info
	var tenant entity.TntTenants
	err = dao.TntTenants.Ctx(ctx).Where("id", user.TenantId).Scan(&tenant)
	if err != nil {
		return nil, err
	}

	res := &v1.Tenant2FAVerifyRes{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    tokenPair.ExpiresAt.Format(time.RFC3339),
	}
	res.User.ID = user.Id
	res.User.Username = user.Username
	res.User.Role = user.Role
	res.Tenant.ID = tenant.Id
	res.Tenant.Name = tenant.Name
	res.Tenant.Code = tenant.Code
	return res, nil
}

// Setup2FA starts the 2FA setup process for the current tenant user.
func (s *sTenant) Setup2FA(ctx context.Context, _ *v1.Tenant2FASetupReq) (*v1.Tenant2FASetupRes, error) {
	userID := ctxUserID(ctx)
	secret, uri, err := common.Setup2FA(ctx, "tenant", userID)
	if err != nil {
		return nil, err
	}

	encKey := getEncKey(ctx)
	encrypted, err := crypto.EncryptString(encKey, secret)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("2fa:pending:tenant:%d", userID)
	_, err = g.Redis().Do(ctx, "SETEX", key, 300, encrypted)
	if err != nil {
		return nil, err
	}

	return &v1.Tenant2FASetupRes{Secret: secret, URI: uri}, nil
}

// Enable2FA confirms and enables 2FA.
func (s *sTenant) Enable2FA(ctx context.Context, req *v1.Tenant2FAEnableReq) (*v1.Tenant2FAEnableRes, error) {
	userID := ctxUserID(ctx)
	secret, err := getPendingTOTPSecret(ctx, userID)
	if err != nil || secret == "" {
		return nil, common.NewBusinessError(consts.CodeBadRequest, "请先执行2FA设置")
	}

	backupCodes, err := common.Enable2FA(ctx, "tenant", userID, secret, req.Code, req.Password)
	if err != nil {
		return nil, err
	}

	clearPendingTOTPSecret(ctx, userID)
	return &v1.Tenant2FAEnableRes{BackupCodes: backupCodes}, nil
}

// Disable2FA disables 2FA for the current tenant user.
func (s *sTenant) Disable2FA(ctx context.Context, req *v1.Tenant2FADisableReq) (*v1.Tenant2FADisableRes, error) {
	userID := ctxUserID(ctx)
	err := common.Disable2FA(ctx, "tenant", userID, req.Code)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// RegenerateBackupCodes generates new backup codes.
func (s *sTenant) RegenerateBackupCodes(ctx context.Context, req *v1.Tenant2FARegenerateBackupCodesReq) (*v1.Tenant2FARegenerateBackupCodesRes, error) {
	userID := ctxUserID(ctx)
	codes, err := common.RegenerateBackupCodes(ctx, "tenant", userID, req.Code)
	if err != nil {
		return nil, err
	}
	return &v1.Tenant2FARegenerateBackupCodesRes{BackupCodes: codes}, nil
}

// ConfirmHighRisk generates a confirm token for high-risk operations.
func (s *sTenant) ConfirmHighRisk(ctx context.Context, req *v1.Tenant2FAConfirmReq) (*v1.Tenant2FAConfirmRes, error) {
	userID := ctxUserID(ctx)

	enabled, err := common.Is2FAEnabled(ctx, "tenant", userID)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, common.NewBusinessError(consts.CodeTotpNotEnabled, consts.MsgTotpNotEnabled)
	}

	valid, err := common.Verify2FACode(ctx, "tenant", userID, req.Code)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, common.NewBusinessError(consts.CodeTotpInvalid, consts.MsgTotpInvalid)
	}

	token, err := common.GenerateConfirmToken(ctx, userID, "tenant")
	if err != nil {
		return nil, err
	}

	return &v1.Tenant2FAConfirmRes{ConfirmToken: token}, nil
}

// LoginHistory returns the login history for the current tenant user.
func (s *sTenant) LoginHistory(ctx context.Context, req *v1.TenantLoginHistoryReq) (*v1.TenantLoginHistoryRes, error) {
	userID := ctxUserID(ctx)
	page := req.Page
	pageSize := req.PageSize
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	total, err := dao.AudLoginHistory.Ctx(ctx).
		Where("user_type", "tenant").
		Where("user_id", userID).
		Count()
	if err != nil {
		return nil, err
	}

	var records []entity.AudLoginHistory
	err = dao.AudLoginHistory.Ctx(ctx).
		Where("user_type", "tenant").
		Where("user_id", userID).
		OrderDesc("created_at").
		Page(page, pageSize).
		Scan(&records)
	if err != nil {
		return nil, err
	}

	items := make([]v1.TenantLoginHistoryItem, len(records))
	for i, r := range records {
		items[i] = v1.TenantLoginHistoryItem{
			ID:          r.Id,
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

	return &v1.TenantLoginHistoryRes{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func getPendingTOTPSecret(ctx context.Context, userID int64) (string, error) {
	key := fmt.Sprintf("2fa:pending:tenant:%d", userID)
	val, err := g.Redis().Do(ctx, "GET", key)
	if err != nil {
		return "", err
	}
	if val.IsNil() || val.IsEmpty() {
		return "", nil
	}
	encKey := getEncKey(ctx)
	return crypto.DecryptString(encKey, val.String())
}

func clearPendingTOTPSecret(ctx context.Context, userID int64) {
	key := fmt.Sprintf("2fa:pending:tenant:%d", userID)
	_, _ = g.Redis().Do(ctx, "DEL", key)
}

func getEncKey(ctx context.Context) []byte {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		hexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	return crypto.MustGetEncryptionKey(hexKey)
}

// GetIPWhitelist returns the tenant's IP whitelist configuration.
func (s *sTenant) GetIPWhitelist(ctx context.Context, _ *v1.TenantIPWhitelistGetReq) (*v1.TenantIPWhitelistGetRes, error) {
	tenantID := ctxTenantID(ctx)
	var tenant entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]any)
	if tenant.Settings != "" {
		_ = json.Unmarshal([]byte(tenant.Settings), &settings)
	}

	enabled := false
	whitelist := []string{}
	if v, ok := settings["ip_whitelist_enabled"]; ok {
		enabled = v.(bool)
	}
	if v, ok := settings["ip_whitelist"]; ok {
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				whitelist = append(whitelist, fmt.Sprintf("%v", item))
			}
		}
	}

	return &v1.TenantIPWhitelistGetRes{Enabled: enabled, Whitelist: whitelist}, nil
}

// UpdateIPWhitelist updates the tenant's IP whitelist configuration.
func (s *sTenant) UpdateIPWhitelist(ctx context.Context, req *v1.TenantIPWhitelistUpdateReq) (*v1.TenantIPWhitelistUpdateRes, error) {
	tenantID := ctxTenantID(ctx)
	var tenant entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil {
		return nil, err
	}

	settings := make(map[string]any)
	if tenant.Settings != "" {
		_ = json.Unmarshal([]byte(tenant.Settings), &settings)
	}

	if req.Enabled != nil {
		settings["ip_whitelist_enabled"] = *req.Enabled
	}
	if req.Whitelist != nil {
		settings["ip_whitelist"] = req.Whitelist
	}

	settingsJSON, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}

	_, err = dao.TntTenants.Ctx(ctx).Where("id", tenantID).Data(g.Map{
		"settings": string(settingsJSON),
	}).Update()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// CheckIPWhitelist checks if the given IP is allowed for the tenant.
// Returns true if allowed (or whitelist is disabled).
func CheckIPWhitelist(ctx context.Context, tenantID int64, ip string) bool {
	var tenant entity.TntTenants
	err := dao.TntTenants.Ctx(ctx).Where("id", tenantID).Scan(&tenant)
	if err != nil || tenant.Settings == "" {
		return true
	}

	settings := make(map[string]any)
	if err := json.Unmarshal([]byte(tenant.Settings), &settings); err != nil {
		return true
	}

	enabled, _ := settings["ip_whitelist_enabled"].(bool)
	if !enabled {
		return true
	}

	whitelist, ok := settings["ip_whitelist"]
	if !ok {
		return true
	}

	arr, ok := whitelist.([]any)
	if !ok || len(arr) == 0 {
		return true
	}

	for _, item := range arr {
		cidr := fmt.Sprintf("%v", item)
		if ip == cidr {
			return true
		}
		// Simple CIDR / prefix match for common cases
		if strings.HasSuffix(cidr, ".*") {
			prefix := strings.TrimSuffix(cidr, ".*")
			if strings.HasPrefix(ip, prefix) {
				return true
			}
		}
	}
	return false
}
