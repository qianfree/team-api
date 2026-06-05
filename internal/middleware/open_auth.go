package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/response"
	"github.com/qianfree/team-api/internal/utility/crypto"
)

// openContextKey is a private type for context keys to prevent collisions.
type openContextKey string

const (
	openPlatformHeaderSignature = "X-Signature"
	openPlatformHeaderTimestamp = "X-Timestamp"
	openPlatformHeaderNonce     = "X-Nonce"
	openPlatformHeaderAppID     = "X-App-Id"
	openPlatformMaxSkew         = 5 * time.Minute

	ctxKeyOpenAppID       openContextKey = "openAppId"
	ctxKeyOpenTenantID    openContextKey = "openTenantId"
	ctxKeyOpenPermissions openContextKey = "openPermissions"
	ctxKeyOpenIsSandbox   openContextKey = "openIsSandbox"
)

// OpenPlatformAuth is the HMAC-SHA256 authentication middleware for /open/* endpoints.
func OpenPlatformAuth(r *ghttp.Request) {
	appID := r.GetHeader(openPlatformHeaderAppID)
	signature := r.GetHeader(openPlatformHeaderSignature)
	timestampStr := r.GetHeader(openPlatformHeaderTimestamp)
	nonce := r.GetHeader(openPlatformHeaderNonce)

	if appID == "" || signature == "" || timestampStr == "" || nonce == "" {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Validate timestamp skew
	ts, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		response.Error(r, consts.ErrUnauthorized)
		return
	}
	if time.Since(time.Unix(ts, 0)).Abs() > openPlatformMaxSkew {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Lookup app by app_id
	var app entity.OpnApps
	err = dao.OpnApps.Ctx(r.Context()).Where("app_id", appID).Scan(&app)
	if err != nil || app.Id == 0 {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	if app.Status != "active" {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Check IP whitelist
	if err := checkOpenAppIPWhitelist(r, app); err != nil {
		response.Error(r, err)
		return
	}

	// Get app secret from Redis
	secret, err := getOpenAppSecret(r.Context(), app.Id)
	if err != nil || secret == "" {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Read request body for signature verification
	var bodyBytes []byte
	if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
		bodyBytes, _ = io.ReadAll(r.Body)
	}

	// Verify HMAC signature (includes body hash for request integrity)
	expectedSig := computeHMACSignature(secret, timestampStr, nonce, r.Method, r.URL.Path, bodyBytes)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Inject context variables using typed keys
	ctx := r.Context()
	ctx = context.WithValue(ctx, ctxKeyOpenAppID, app.Id)
	ctx = context.WithValue(ctx, ctxKeyOpenTenantID, app.TenantId)
	ctx = context.WithValue(ctx, ctxKeyOpenPermissions, app.Permissions)
	ctx = context.WithValue(ctx, ctxKeyOpenIsSandbox, app.IsSandbox)
	r.SetCtx(ctx)

	r.Middleware.Next()
}

func computeHMACSignature(secret, timestamp, nonce, method, path string, body []byte) string {
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", timestamp, nonce, method, path, bodyHashHex)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func checkOpenAppIPWhitelist(r *ghttp.Request, app entity.OpnApps) error {
	if app.IpWhitelist == "" || app.IpWhitelist == "[]" {
		return nil
	}
	var whitelist []string
	_ = json.Unmarshal([]byte(app.IpWhitelist), &whitelist)
	if len(whitelist) == 0 {
		return nil
	}

	clientIP := r.GetClientIp()
	for _, allowed := range whitelist {
		if clientIP == allowed {
			return nil
		}
		if strings.HasSuffix(allowed, ".*") {
			prefix := strings.TrimSuffix(allowed, ".*")
			if strings.HasPrefix(clientIP, prefix) {
				return nil
			}
		}
	}
	return consts.ErrIpRestricted
}

func getOpenAppSecret(ctx context.Context, appID int64) (string, error) {
	val, err := g.Redis().Do(ctx, "GET", fmt.Sprintf("open:secret:%d", appID))
	if err != nil {
		return "", err
	}
	if val.IsNil() || val.IsEmpty() {
		return "", fmt.Errorf("secret not found")
	}
	encKey := getEncKey(ctx)
	return crypto.DecryptString(encKey, val.String())
}

func getEncKey(ctx context.Context) []byte {
	hexKey := g.Cfg().MustGet(ctx, "crypto.encryptionKey").String()
	if hexKey == "" {
		panic("crypto.encryptionKey is not configured — refusing to start with weak key")
	}
	return crypto.MustGetEncryptionKey(hexKey)
}

// GetOpenTenantID extracts the tenant ID injected by OpenPlatformAuth middleware.
func GetOpenTenantID(ctx context.Context) int64 {
	val := ctx.Value(ctxKeyOpenTenantID)
	if val != nil {
		if id, ok := val.(int64); ok {
			return id
		}
	}
	return 0
}

// GetOpenAppID extracts the app ID injected by OpenPlatformAuth middleware.
func GetOpenAppID(ctx context.Context) int64 {
	val := ctx.Value(ctxKeyOpenAppID)
	if val != nil {
		if id, ok := val.(int64); ok {
			return id
		}
	}
	return 0
}
