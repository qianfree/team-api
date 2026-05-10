package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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

const (
	openPlatformHeaderSignature = "X-Signature"
	openPlatformHeaderTimestamp = "X-Timestamp"
	openPlatformHeaderNonce     = "X-Nonce"
	openPlatformHeaderAppID     = "X-App-Id"
	openPlatformMaxSkew         = 5 * time.Minute
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

	// Verify HMAC signature
	expectedSig := computeHMACSignature(secret, timestampStr, nonce, r.Method, r.URL.Path)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Inject context variables
	ctx := r.Context()
	ctx = context.WithValue(ctx, "openAppId", app.Id)
	ctx = context.WithValue(ctx, "openTenantId", app.TenantId)
	ctx = context.WithValue(ctx, "openPermissions", app.Permissions)
	ctx = context.WithValue(ctx, "openIsSandbox", app.IsSandbox)
	r.SetCtx(ctx)

	r.Middleware.Next()
}

func computeHMACSignature(secret, timestamp, nonce, method, path string) string {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", timestamp, nonce, method, path)
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
		hexKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	}
	return crypto.MustGetEncryptionKey(hexKey)
}
