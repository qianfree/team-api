package common

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func init() {
	jwtSecret = []byte("test-secret-for-unit-tests-only!!")
}

// ─── HashRefreshToken ───────────────────────────────────────────────

func TestHashRefreshToken_Deterministic(t *testing.T) {
	token := "abc123def456"
	h1 := HashRefreshToken(token)
	h2 := HashRefreshToken(token)
	if h1 != h2 {
		t.Fatalf("same input produced different hashes: %s vs %s", h1, h2)
	}
}

func TestHashRefreshToken_DifferentInputs(t *testing.T) {
	h1 := HashRefreshToken("token-a")
	h2 := HashRefreshToken("token-b")
	if h1 == h2 {
		t.Fatal("different inputs produced the same hash")
	}
}

func TestHashRefreshToken_Length(t *testing.T) {
	h := HashRefreshToken("some-token")
	if len(h) != 64 {
		t.Fatalf("expected SHA-256 hex length 64, got %d", len(h))
	}
}

func TestHashRefreshToken_Empty(t *testing.T) {
	h := HashRefreshToken("")
	if len(h) != 64 {
		t.Fatalf("expected SHA-256 hex length 64 for empty input, got %d", len(h))
	}
}

// ─── GenerateRefreshToken ───────────────────────────────────────────

func TestGenerateRefreshToken_Length(t *testing.T) {
	token, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(token) != 64 {
		t.Fatalf("expected 64-char hex string, got %d chars", len(token))
	}
}

func TestGenerateRefreshToken_Unique(t *testing.T) {
	t1, _ := GenerateRefreshToken()
	t2, _ := GenerateRefreshToken()
	if t1 == t2 {
		t.Fatal("two generated tokens should be different")
	}
}

// ─── GenerateTokenPair + ParseAccessToken round-trip ────────────────

func TestTokenPairRoundTrip(t *testing.T) {
	pair, err := generateTestTokenPair(42, "admin", "super_admin", 100, 1)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pair.AccessToken == "" {
		t.Fatal("access token is empty")
	}
	if pair.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}
	if pair.ExpiresAt.Before(time.Now()) {
		t.Fatal("expires_at is in the past")
	}

	claims, err := ParseAccessToken(pair.AccessToken)
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
	if claims.TenantID != 100 {
		t.Fatalf("TenantID = %d, want 100", claims.TenantID)
	}
	if claims.SessionID != 1 {
		t.Fatalf("SessionID = %d, want 1", claims.SessionID)
	}
}

func TestTokenPairRoundTrip_TenantUser(t *testing.T) {
	pair, err := generateTestTokenPair(99, "tenant", "member", 200, 5)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := ParseAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserType != "tenant" {
		t.Fatalf("UserType = %q, want tenant", claims.UserType)
	}
	if claims.TenantID != 200 {
		t.Fatalf("TenantID = %d, want 200", claims.TenantID)
	}
}

// ─── ParseAccessToken error cases ───────────────────────────────────

func TestParseAccessToken_InvalidString(t *testing.T) {
	_, err := ParseAccessToken("not-a-jwt")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestParseAccessToken_Empty(t *testing.T) {
	_, err := ParseAccessToken("")
	if err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestParseAccessToken_Expired(t *testing.T) {
	now := time.Now().Add(-10 * time.Minute)
	claims := JWTClaims{
		UserID:    1,
		UserType:  "admin",
		Role:      "admin",
		SessionID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString(GetJWTSecret())

	_, err := ParseAccessToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParseAccessToken_WrongSecret(t *testing.T) {
	claims := JWTClaims{
		UserID:    1,
		UserType:  "admin",
		Role:      "admin",
		SessionID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("wrong-secret"))

	_, err := ParseAccessToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for token signed with wrong secret")
	}
}

func TestParseAccessToken_RejectsNoneAlgorithm(t *testing.T) {
	claims := JWTClaims{
		UserID:   1,
		UserType: "admin",
		Role:     "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	tokenStr, _ := token.SignedString(jwt.UnsafeAllowNoneSignatureType)

	_, err := ParseAccessToken(tokenStr)
	if err == nil {
		t.Fatal("expected error for 'none' signing method")
	}
}

// ─── RefreshToken hash consistency ──────────────────────────────────

func TestRefreshTokenHashIsNotReversible(t *testing.T) {
	token, _ := GenerateRefreshToken()
	hash := HashRefreshToken(token)
	if strings.Contains(hash, token) {
		t.Fatal("hash should not contain the original token")
	}
}

// ─── helpers ────────────────────────────────────────────────────────

func generateTestTokenPair(userID int64, userType, role string, tenantID, sessionID int64) (*TokenPair, error) {
	now := time.Now()
	accessClaims := JWTClaims{
		UserID:    userID,
		UserType:  userType,
		Role:      role,
		TenantID:  tenantID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "team-api-test",
			Subject:   "42",
			ExpiresAt: jwt.NewNumericDate(now.Add(30 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(GetJWTSecret())
	if err != nil {
		return nil, err
	}

	refreshTokenStr, err := GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    now.Add(30 * time.Minute),
	}, nil
}
