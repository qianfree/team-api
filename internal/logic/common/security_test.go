package common

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ─── ProvisionalToken round-trip ────────────────────────────────────

func TestProvisionalToken_RoundTrip(t *testing.T) {
	token, err := generateTestProvisionalToken(42, "admin", "super_admin", 0)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ParseProvisionalToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
	if claims.UserType != "admin" {
		t.Fatalf("UserType = %q, want admin", claims.UserType)
	}
	if claims.Role != "super_admin" {
		t.Fatalf("Role = %q, want super_admin", claims.Role)
	}
	if claims.Purpose != "totp_verify" {
		t.Fatalf("Purpose = %q, want totp_verify", claims.Purpose)
	}
}

func TestProvisionalToken_TenantUser(t *testing.T) {
	token, err := generateTestProvisionalToken(99, "tenant", "member", 200)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ParseProvisionalToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.TenantID != 200 {
		t.Fatalf("TenantID = %d, want 200", claims.TenantID)
	}
}

func TestProvisionalToken_Expired(t *testing.T) {
	now := time.Now().Add(-10 * time.Minute)
	claims := ProvisionalClaims{
		UserID:   1,
		UserType: "admin",
		Role:     "admin",
		Purpose:  "totp_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api",
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(GetJWTSecret())

	_, err := ParseProvisionalToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired provisional token")
	}
}

func TestProvisionalToken_WrongPurpose(t *testing.T) {
	now := time.Now()
	claims := ProvisionalClaims{
		UserID:   1,
		UserType: "admin",
		Role:     "admin",
		Purpose:  "high_risk_confirm",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(GetJWTSecret())

	_, err := ParseProvisionalToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong purpose in provisional token")
	}
}

func TestProvisionalToken_InvalidString(t *testing.T) {
	_, err := ParseProvisionalToken("garbage")
	if err == nil {
		t.Fatal("expected error for invalid token string")
	}
}

// ─── ConfirmToken round-trip ────────────────────────────────────────

func TestConfirmToken_RoundTrip(t *testing.T) {
	token, err := generateTestConfirmToken(42, "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ParseConfirmToken(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
	if claims.UserType != "admin" {
		t.Fatalf("UserType = %q, want admin", claims.UserType)
	}
	if claims.Purpose != "high_risk_confirm" {
		t.Fatalf("Purpose = %q, want high_risk_confirm", claims.Purpose)
	}
}

func TestConfirmToken_Expired(t *testing.T) {
	now := time.Now().Add(-10 * time.Minute)
	claims := ConfirmClaims{
		UserID:   1,
		UserType: "admin",
		Purpose:  "high_risk_confirm",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(GetJWTSecret())

	_, err := ParseConfirmToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired confirm token")
	}
}

func TestConfirmToken_WrongPurpose(t *testing.T) {
	now := time.Now()
	claims := ConfirmClaims{
		UserID:   1,
		UserType: "admin",
		Purpose:  "totp_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(GetJWTSecret())

	_, err := ParseConfirmToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for wrong purpose in confirm token")
	}
}

func TestConfirmToken_InvalidString(t *testing.T) {
	_, err := ParseConfirmToken("")
	if err == nil {
		t.Fatal("expected error for empty confirm token")
	}
}

// ─── Cross-token type rejection ─────────────────────────────────────

func TestProvisionalTokenCannotParseAsConfirm(t *testing.T) {
	provToken, _ := generateTestProvisionalToken(1, "admin", "admin", 0)
	_, err := ParseConfirmToken(provToken)
	if err == nil {
		t.Fatal("provisional token should not parse as confirm token")
	}
}

func TestConfirmTokenCannotParseAsProvisional(t *testing.T) {
	confToken, _ := generateTestConfirmToken(1, "admin")
	_, err := ParseProvisionalToken(confToken)
	if err == nil {
		t.Fatal("confirm token should not parse as provisional token")
	}
}

// ─── DeviceFingerprint ──────────────────────────────────────────────

func TestDeviceFingerprint_Deterministic(t *testing.T) {
	fp1 := DeviceFingerprint("", "Mozilla/5.0")
	fp2 := DeviceFingerprint("", "Mozilla/5.0")
	if fp1 != fp2 {
		t.Fatalf("same input produced different fingerprints: %q vs %q", fp1, fp2)
	}
}

func TestDeviceFingerprint_DifferentUA(t *testing.T) {
	fp1 := DeviceFingerprint("", "Chrome/120")
	fp2 := DeviceFingerprint("", "Firefox/115")
	if fp1 == fp2 {
		t.Fatal("different UAs should produce different fingerprints")
	}
}

func TestDeviceFingerprint_DeviceIDPreferred(t *testing.T) {
	// 有设备 ID 时优先走 dev: 分支，UA 差异不影响指纹
	fp1 := DeviceFingerprint("device-abc", "Chrome/120")
	fp2 := DeviceFingerprint("device-abc", "Firefox/115")
	if fp1 != fp2 {
		t.Fatal("same device ID should produce same fingerprint regardless of UA")
	}
	if !strings.HasPrefix(fp1, "dev:") {
		t.Fatalf("device ID fingerprint should have dev: prefix, got %q", fp1)
	}

	fp3 := DeviceFingerprint("device-xyz", "Chrome/120")
	if fp1 == fp3 {
		t.Fatal("different device IDs should produce different fingerprints")
	}
}

func TestDeviceFingerprint_FallbackPrefix(t *testing.T) {
	fp := DeviceFingerprint("", "Mozilla/5.0")
	if !strings.HasPrefix(fp, "ua:") {
		t.Fatalf("UA fallback fingerprint should have ua: prefix, got %q", fp)
	}
}

func TestDeviceFingerprint_Length(t *testing.T) {
	// dev: 前缀 4 字符 + 32 hex = 36；ua: 前缀 3 字符 + 32 hex = 35
	if fp := DeviceFingerprint("some-device", ""); len(fp) != 36 {
		t.Fatalf("expected length 36 for device ID branch, got %d", len(fp))
	}
	if fp := DeviceFingerprint("", "some-ua"); len(fp) != 35 {
		t.Fatalf("expected length 35 for UA fallback branch, got %d", len(fp))
	}
}

func TestDeviceFingerprint_CaseInsensitive(t *testing.T) {
	fp1 := DeviceFingerprint("", "Mozilla/5.0")
	fp2 := DeviceFingerprint("", "MOZILLA/5.0")
	if fp1 != fp2 {
		t.Fatal("fingerprint should be case-insensitive on UA")
	}
}

func TestDeviceFingerprint_LongUA(t *testing.T) {
	longUA := strings.Repeat("A", 500)
	fp := DeviceFingerprint("", longUA)
	if len(fp) != 35 {
		t.Fatalf("expected length 35 for long UA, got %d", len(fp))
	}

	truncatedUA := strings.Repeat("A", 200)
	fpTruncated := DeviceFingerprint("", truncatedUA)
	if fp != fpTruncated {
		t.Fatal("UA beyond 200 chars should not affect fingerprint")
	}
}

func TestDeviceFingerprint_LongDeviceID(t *testing.T) {
	longID := strings.Repeat("d", 500)
	fp := DeviceFingerprint(longID, "")
	if len(fp) != 36 {
		t.Fatalf("expected length 36 for long device ID, got %d", len(fp))
	}

	truncatedID := strings.Repeat("d", 128)
	fpTruncated := DeviceFingerprint(truncatedID, "")
	if fp != fpTruncated {
		t.Fatal("device ID beyond 128 chars should not affect fingerprint")
	}
}

func TestDeviceFingerprint_Empty(t *testing.T) {
	fp := DeviceFingerprint("", "")
	if len(fp) != 35 {
		t.Fatalf("expected length 35 for empty input, got %d", len(fp))
	}
}

func TestExtractDeviceInfoWithoutRequest(t *testing.T) {
	var info map[string]string
	if err := json.Unmarshal([]byte(ExtractDeviceInfo(context.Background())), &info); err != nil {
		t.Fatalf("ExtractDeviceInfo returned invalid JSON: %v", err)
	}
	if info["user_agent"] != "unknown" {
		t.Fatalf("user_agent = %q, want unknown", info["user_agent"])
	}
}

func TestBuildDeviceInfoTruncatesUserAgent(t *testing.T) {
	var info map[string]string
	if err := json.Unmarshal([]byte(buildDeviceInfo(strings.Repeat("a", 501))), &info); err != nil {
		t.Fatalf("buildDeviceInfo returned invalid JSON: %v", err)
	}
	if len(info["user_agent"]) != 500 {
		t.Fatalf("user_agent length = %d, want 500", len(info["user_agent"]))
	}
}

func TestBackupCodeSHA256(t *testing.T) {
	const code = "ABCD-EFGH"
	hashed := hashBackupCode(code)
	if !strings.HasPrefix(hashed, backupCodeSHA256Prefix) {
		t.Fatalf("backup code hash %q is missing algorithm prefix", hashed)
	}
	if !verifyBackupCode(code, hashed) {
		t.Fatal("correct backup code did not verify")
	}
	if verifyBackupCode("wrong-code", hashed) {
		t.Fatal("incorrect backup code verified")
	}
}

// ─── helpers ────────────────────────────────────────────────────────

func generateTestProvisionalToken(userID int64, userType, role string, tenantID int64) (string, error) {
	now := time.Now()
	claims := ProvisionalClaims{
		UserID:   userID,
		UserType: userType,
		Role:     role,
		TenantID: tenantID,
		Purpose:  "totp_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api",
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}

func generateTestConfirmToken(userID int64, userType string) (string, error) {
	now := time.Now()
	claims := ConfirmClaims{
		UserID:   userID,
		UserType: userType,
		Purpose:  "high_risk_confirm",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api",
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(GetJWTSecret())
}
