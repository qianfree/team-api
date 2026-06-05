package common

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims defines the payload of the access token.
type JWTClaims struct {
	UserID    int64  `json:"user_id"`
	UserType  string `json:"user_type"` // admin / tenant
	Role      string `json:"role"`
	TenantID  int64  `json:"tenant_id,omitempty"`
	SessionID int64  `json:"session_id"`
	jwt.RegisteredClaims
}

// TokenPair holds an access token and a refresh token.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

var jwtSecret []byte

// InitJWTSecret loads the JWT signing secret from config.
func InitJWTSecret(ctx context.Context) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	if secret == "" {
		panic("jwt.secret is not configured — refusing to start with weak key")
	}
	jwtSecret = []byte(secret)
}

// GetJWTSecret returns the cached JWT signing secret.
func GetJWTSecret() []byte {
	if jwtSecret == nil {
		panic("JWT secret not initialized — call InitJWTSecret first")
	}
	return jwtSecret
}

// getAccessExpire returns the access token expiration duration.
func getAccessExpire(ctx context.Context) time.Duration {
	d := g.Cfg().MustGet(ctx, "jwt.accessTokenExpire").String()
	if d == "" {
		return 30 * time.Minute
	}
	dur, err := time.ParseDuration(d)
	if err != nil {
		return 30 * time.Minute
	}
	return dur
}

// getRefreshExpire returns the refresh token expiration duration.
func getRefreshExpire(ctx context.Context) time.Duration {
	d := g.Cfg().MustGet(ctx, "jwt.refreshTokenExpire").String()
	if d == "" {
		return 7 * 24 * time.Hour
	}
	dur, err := time.ParseDuration(d)
	if err != nil {
		return 7 * 24 * time.Hour
	}
	return dur
}

// getIssuer returns the JWT issuer.
func getIssuer(ctx context.Context) string {
	return g.Cfg().MustGet(ctx, "jwt.issuer").String()
}

// GenerateTokenPair creates an access token and a refresh token.
// jti is a UUID that uniquely identifies the session for revocation tracking.
func GenerateTokenPair(ctx context.Context, userID int64, userType, role string, tenantID, sessionID int64, jti string) (*TokenPair, error) {
	now := time.Now()
	accessExpire := getAccessExpire(ctx)
	issuer := getIssuer(ctx)

	// Access token
	accessClaims := JWTClaims{
		UserID:    userID,
		UserType:  userType,
		Role:      role,
		TenantID:  tenantID,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessExpire)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenStr, err := accessToken.SignedString(GetJWTSecret())
	if err != nil {
		return nil, gerror.Wrapf(err, "sign access token")
	}

	// Refresh token (random string, not JWT)
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, gerror.Wrapf(err, "generate refresh token")
	}
	refreshTokenStr := hex.EncodeToString(refreshBytes)

	return &TokenPair{
		AccessToken:  accessTokenStr,
		RefreshToken: refreshTokenStr,
		ExpiresAt:    now.Add(accessExpire),
	}, nil
}

// ParseAccessToken parses and validates an access token.
func ParseAccessToken(tokenStr string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, gerror.Newf("unexpected signing method: %v", token.Header["alg"])
		}
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, gerror.New("invalid token claims")
	}

	return claims, nil
}

// HashRefreshToken returns the SHA-256 hash of a refresh token (for database storage).
func HashRefreshToken(refreshToken string) string {
	hash := sha256.Sum256([]byte(refreshToken))
	return hex.EncodeToString(hash[:])
}

// GenerateRefreshToken generates a new random refresh token string.
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", gerror.Wrapf(err, "generate refresh token")
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateJti generates a new UUID-based JWT ID for session revocation tracking.
func GenerateJti() string {
	return uuid.New().String()
}

// GetMaxSessions returns the max concurrent sessions for a user type.
func GetMaxSessions(ctx context.Context, userType string) int {
	if userType == "admin" {
		return g.Cfg().MustGet(ctx, "jwt.adminMaxSessions").Int()
	}
	return g.Cfg().MustGet(ctx, "jwt.tenantMaxSessions").Int()
}
