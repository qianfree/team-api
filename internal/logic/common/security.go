package common

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/golang-jwt/jwt/v5"

	"github.com/qianfree/team-api/internal/dao"
	do "github.com/qianfree/team-api/internal/model/do"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/utility/crypto"
	"github.com/qianfree/team-api/internal/utility/totp"
)

const provisionalTTL = 5 * time.Minute
const confirmTokenTTL = 5 * time.Minute

// ProvisionalClaims is a short-lived JWT for 2FA pending state.
type ProvisionalClaims struct {
	UserID   int64  `json:"user_id"`
	UserType string `json:"user_type"`
	Role     string `json:"role"`
	TenantID int64  `json:"tenant_id,omitempty"`
	Purpose  string `json:"purpose"` // "totp_verify"
	jwt.RegisteredClaims
}

// ConfirmClaims is a short-lived JWT for high-risk operation confirmation.
type ConfirmClaims struct {
	UserID   int64  `json:"user_id"`
	UserType string `json:"user_type"`
	Purpose  string `json:"purpose"` // "high_risk_confirm"
	jwt.RegisteredClaims
}

// GenerateProvisionalToken generates a short-lived token for 2FA verification step.
func GenerateProvisionalToken(ctx context.Context, userID int64, userType, role string, tenantID int64) (string, error) {
	now := time.Now()
	claims := ProvisionalClaims{
		UserID:   userID,
		UserType: userType,
		Role:     role,
		TenantID: tenantID,
		Purpose:  "totp_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api",
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(now.Add(provisionalTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

// ParseProvisionalToken parses and validates a provisional 2FA token.
func ParseProvisionalToken(tokenStr string) (*ProvisionalClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &ProvisionalClaims{}, func(token *jwt.Token) (any, error) {
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*ProvisionalClaims)
	if !ok || !token.Valid || claims.Purpose != "totp_verify" {
		return nil, fmt.Errorf("invalid provisional token")
	}
	return claims, nil
}

// GenerateConfirmToken generates a short-lived token for high-risk operation confirmation.
func GenerateConfirmToken(ctx context.Context, userID int64, userType string) (string, error) {
	now := time.Now()
	claims := ConfirmClaims{
		UserID:   userID,
		UserType: userType,
		Purpose:  "high_risk_confirm",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api",
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(now.Add(confirmTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

// ParseConfirmToken parses and validates a high-risk confirm token.
func ParseConfirmToken(tokenStr string) (*ConfirmClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &ConfirmClaims{}, func(token *jwt.Token) (any, error) {
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*ConfirmClaims)
	if !ok || !token.Valid || claims.Purpose != "high_risk_confirm" {
		return nil, fmt.Errorf("invalid confirm token")
	}
	return claims, nil
}

// Setup2FA generates a new TOTP secret for a user. Returns the secret and otpauth URI.
// Callers must still persist the secret after the user confirms with a valid code.
func Setup2FA(ctx context.Context, userType string, userID int64) (secret, uri string, err error) {
	var accountName string
	if userType == "admin" {
		var user *entity.SysAdminUsers
		err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return "", "", err
		}
		if user == nil {
			return "", "", NewBusinessError(10024, "用户不存在")
		}
		accountName = user.Username
	} else {
		var user *entity.TntUsers
		err = dao.TntUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return "", "", err
		}
		if user == nil {
			return "", "", NewBusinessError(10024, "用户不存在")
		}
		accountName = fmt.Sprintf("%s@%d", user.Username, user.TenantId)
	}

	secret, err = totp.GenerateSecret(accountName)
	if err != nil {
		return "", "", err
	}

	uri = totp.GenerateURI(accountName, secret)
	return secret, uri, nil
}

// Enable2FA enables 2FA for a user after verifying the TOTP code and password.
// Stores the encrypted secret and hashed backup codes.
func Enable2FA(ctx context.Context, userType string, userID int64, secret, code, password string) ([]string, error) {
	// Verify TOTP code
	if !totp.ValidateCode(code, secret) {
		return nil, NewBusinessError(10048, "验证码错误")
	}

	// Verify current password
	var passwordHash string
	if userType == "admin" {
		var user *entity.SysAdminUsers
		err := dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, NewBusinessError(10024, "用户不存在")
		}
		passwordHash = user.PasswordHash
	} else {
		var user *entity.TntUsers
		err := dao.TntUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, NewBusinessError(10024, "用户不存在")
		}
		passwordHash = user.PasswordHash
	}

	if !crypto.VerifyPassword(password, passwordHash) {
		return nil, NewBusinessError(10023, "密码错误")
	}

	// Generate backup codes
	plainCodes, err := totp.GenerateBackupCodes(8)
	if err != nil {
		return nil, err
	}

	// Hash backup codes for storage
	hashedCodes := make([]string, len(plainCodes))
	for i, code := range plainCodes {
		hash, _ := crypto.HashPassword(code)
		hashedCodes[i] = hash
	}
	codesJSON, _ := json.Marshal(hashedCodes)

	// Encrypt TOTP secret
	encKey := getEncryptionKey(ctx)
	encryptedSecret, err := crypto.EncryptString(encKey, secret)
	if err != nil {
		return nil, err
	}

	// Persist
	if userType == "admin" {
		_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Data(do.SysAdminUsers{
			TotpSecret:  encryptedSecret,
			TotpEnabled: true,
			BackupCodes: string(codesJSON),
		}).Update()
	} else {
		_, err = dao.TntUsers.Ctx(ctx).Where("id", userID).Data(do.TntUsers{
			TotpSecret:  encryptedSecret,
			TotpEnabled: true,
			BackupCodes: string(codesJSON),
		}).Update()
	}
	if err != nil {
		return nil, err
	}

	return plainCodes, nil
}

// Disable2FA disables 2FA for a user after verifying the provided code.
func Disable2FA(ctx context.Context, userType string, userID int64, code string) error {
	secret, _, err := GetUserTOTPSecret(ctx, userType, userID)
	if err != nil {
		return err
	}

	// Verify TOTP code or backup code
	if !totp.ValidateCode(code, secret) {
		matched, err := verifyAndConsumeBackupCode(ctx, userType, userID, code)
		if err != nil || !matched {
			return NewBusinessError(10048, "验证码或恢复码错误")
		}
	}

	if userType == "admin" {
		_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Data(do.SysAdminUsers{
			TotpSecret:  "",
			TotpEnabled: false,
			BackupCodes: "",
		}).Update()
	} else {
		_, err = dao.TntUsers.Ctx(ctx).Where("id", userID).Data(do.TntUsers{
			TotpSecret:  "",
			TotpEnabled: false,
			BackupCodes: "",
		}).Update()
	}
	return err
}

// Verify2FACode verifies a TOTP code or backup code for a user.
// Returns true if valid. If a backup code was used, it's consumed (removed).
func Verify2FACode(ctx context.Context, userType string, userID int64, code string) (bool, error) {
	secret, backupCodes, err := GetUserTOTPSecret(ctx, userType, userID)
	if err != nil {
		return false, err
	}

	// Try TOTP code first
	if totp.ValidateCode(code, secret) {
		return true, nil
	}

	// Try backup codes
	if backupCodes != nil {
		for i, hashedCode := range backupCodes {
			if crypto.VerifyPassword(code, hashedCode) {
				// Consume this backup code
				_ = consumeBackupCode(ctx, userType, userID, i, backupCodes)
				return true, nil
			}
		}
	}

	return false, nil
}

// GetUserTOTPSecret decrypts and returns the TOTP secret and backup codes for a user.
func GetUserTOTPSecret(ctx context.Context, userType string, userID int64) (secret string, backupCodes []string, err error) {
	encKey := getEncryptionKey(ctx)

	if userType == "admin" {
		var user *entity.SysAdminUsers
		err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return "", nil, err
		}
		if user == nil {
			return "", nil, NewBusinessError(10024, "用户不存在")
		}
		if user.TotpSecret != "" {
			secret, err = crypto.DecryptString(encKey, user.TotpSecret)
			if err != nil {
				return "", nil, err
			}
		}
		if user.BackupCodes != "" {
			_ = json.Unmarshal([]byte(user.BackupCodes), &backupCodes)
		}
	} else {
		var user *entity.TntUsers
		err = dao.TntUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return "", nil, err
		}
		if user == nil {
			return "", nil, NewBusinessError(10024, "用户不存在")
		}
		if user.TotpSecret != "" {
			secret, err = crypto.DecryptString(encKey, user.TotpSecret)
			if err != nil {
				return "", nil, err
			}
		}
		if user.BackupCodes != "" {
			_ = json.Unmarshal([]byte(user.BackupCodes), &backupCodes)
		}
	}
	return secret, backupCodes, nil
}

// RegenerateBackupCodes generates new backup codes for a user after verifying TOTP.
func RegenerateBackupCodes(ctx context.Context, userType string, userID int64, code string) ([]string, error) {
	secret, _, err := GetUserTOTPSecret(ctx, userType, userID)
	if err != nil {
		return nil, err
	}

	if !totp.ValidateCode(code, secret) {
		return nil, NewBusinessError(10048, "验证码错误")
	}

	plainCodes, err := totp.GenerateBackupCodes(8)
	if err != nil {
		return nil, err
	}

	hashedCodes := make([]string, len(plainCodes))
	for i, c := range plainCodes {
		hash, _ := crypto.HashPassword(c)
		hashedCodes[i] = hash
	}
	codesJSON, _ := json.Marshal(hashedCodes)

	if userType == "admin" {
		_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Data(do.SysAdminUsers{
			BackupCodes: string(codesJSON),
		}).Update()
	} else {
		_, err = dao.TntUsers.Ctx(ctx).Where("id", userID).Data(do.TntUsers{
			BackupCodes: string(codesJSON),
		}).Update()
	}
	if err != nil {
		return nil, err
	}

	return plainCodes, nil
}

// Is2FAEnabled checks if 2FA is enabled for a user.
func Is2FAEnabled(ctx context.Context, userType string, userID int64) (bool, error) {
	if userType == "admin" {
		var user *entity.SysAdminUsers
		err := dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Scan(&user)
		if err != nil {
			return false, err
		}
		if user == nil {
			return false, nil
		}
		return user.TotpEnabled, nil
	}
	var user *entity.TntUsers
	err := dao.TntUsers.Ctx(ctx).Where("id", userID).Scan(&user)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}
	return user.TotpEnabled, nil
}

// RecordLoginHistory records a login attempt in aud_login_history.
func RecordLoginHistory(ctx context.Context, userType string, userID, tenantID int64, method, ip, ua, deviceFP string, success bool, failReason string) error {
	r := g.RequestFromCtx(ctx)
	isNewDevice := false

	// Check if this device has been seen before
	if deviceFP != "" && success {
		count, err := dao.AudLoginHistory.Ctx(ctx).
			Where("user_type", userType).
			Where("user_id", userID).
			Where("device_fingerprint", deviceFP).
			Where("success", true).
			Count()
		if err == nil && count == 0 {
			isNewDevice = true
		}
	}

	if r != nil {
		_, err := dao.AudLoginHistory.Ctx(ctx).Data(do.AudLoginHistory{
			UserType:          userType,
			UserId:            userID,
			TenantId:          tenantID,
			LoginMethod:       method,
			IpAddress:         ip,
			UserAgent:         ua,
			DeviceFingerprint: deviceFP,
			IsNewDevice:       isNewDevice,
			Success:           success,
			FailReason:        failReason,
		}).Insert()
		return err
	}
	return nil
}

// DeviceFingerprint generates a simple device fingerprint from User-Agent + IP.
func DeviceFingerprint(ua, ip string) string {
	// Simple fingerprint: hash of normalized UA + IP prefix
	normalized := strings.ToLower(ua)
	if len(normalized) > 200 {
		normalized = normalized[:200]
	}
	data := normalized + "|" + ip
	hash := sha256Sum(data)
	return hash[:32]
}

// verifyAndConsumeBackupCode checks if code matches any backup code and marks it consumed.
func verifyAndConsumeBackupCode(ctx context.Context, userType string, userID int64, code string) (bool, error) {
	_, backupCodes, err := GetUserTOTPSecret(ctx, userType, userID)
	if err != nil || backupCodes == nil {
		return false, err
	}

	for i, hashedCode := range backupCodes {
		if crypto.VerifyPassword(code, hashedCode) {
			_ = consumeBackupCode(ctx, userType, userID, i, backupCodes)
			return true, nil
		}
	}
	return false, nil
}

// consumeBackupCode removes a used backup code from the list.
func consumeBackupCode(ctx context.Context, userType string, userID int64, index int, codes []string) error {
	newCodes := make([]string, 0, len(codes)-1)
	for i, c := range codes {
		if i != index {
			newCodes = append(newCodes, c)
		}
	}
	codesJSON, _ := json.Marshal(newCodes)

	var err error
	if userType == "admin" {
		_, err = dao.SysAdminUsers.Ctx(ctx).Where("id", userID).Data(do.SysAdminUsers{
			BackupCodes: string(codesJSON),
		}).Update()
	} else {
		_, err = dao.TntUsers.Ctx(ctx).Where("id", userID).Data(do.TntUsers{
			BackupCodes: string(codesJSON),
		}).Update()
	}
	return err
}

func getEncryptionKey(ctx context.Context) []byte {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		hexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	return crypto.MustGetEncryptionKey(hexKey)
}

func sha256Sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
