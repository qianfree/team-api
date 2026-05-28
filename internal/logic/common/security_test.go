package common

import (
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
	fp1 := DeviceFingerprint("Mozilla/5.0", "192.168.1.1")
	fp2 := DeviceFingerprint("Mozilla/5.0", "192.168.1.1")
	if fp1 != fp2 {
		t.Fatalf("same input produced different fingerprints: %q vs %q", fp1, fp2)
	}
}

func TestDeviceFingerprint_DifferentUA(t *testing.T) {
	fp1 := DeviceFingerprint("Chrome/120", "192.168.1.1")
	fp2 := DeviceFingerprint("Firefox/115", "192.168.1.1")
	if fp1 == fp2 {
		t.Fatal("different UAs should produce different fingerprints")
	}
}

func TestDeviceFingerprint_DifferentIP(t *testing.T) {
	fp1 := DeviceFingerprint("Chrome/120", "10.0.0.1")
	fp2 := DeviceFingerprint("Chrome/120", "10.0.0.2")
	if fp1 == fp2 {
		t.Fatal("different IPs should produce different fingerprints")
	}
}

func TestDeviceFingerprint_Length(t *testing.T) {
	fp := DeviceFingerprint("some-ua", "1.2.3.4")
	if len(fp) != 32 {
		t.Fatalf("expected length 32, got %d", len(fp))
	}
}

func TestDeviceFingerprint_CaseInsensitive(t *testing.T) {
	fp1 := DeviceFingerprint("Mozilla/5.0", "10.0.0.1")
	fp2 := DeviceFingerprint("MOZILLA/5.0", "10.0.0.1")
	if fp1 != fp2 {
		t.Fatal("fingerprint should be case-insensitive on UA")
	}
}

func TestDeviceFingerprint_LongUA(t *testing.T) {
	longUA := strings.Repeat("A", 500)
	fp := DeviceFingerprint(longUA, "1.2.3.4")
	if len(fp) != 32 {
		t.Fatalf("expected length 32 for long UA, got %d", len(fp))
	}

	truncatedUA := strings.Repeat("A", 200)
	fpTruncated := DeviceFingerprint(truncatedUA, "1.2.3.4")
	if fp != fpTruncated {
		t.Fatal("UA beyond 200 chars should not affect fingerprint")
	}
}

func TestDeviceFingerprint_Empty(t *testing.T) {
	fp := DeviceFingerprint("", "")
	if len(fp) != 32 {
		t.Fatalf("expected length 32 for empty input, got %d", len(fp))
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
