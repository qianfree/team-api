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

	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"
	"github.com/qianfree/team-api/internal/response"
	"github.com/qianfree/team-api/internal/utility/crypto"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
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

var openAppCache = lcommon.NewCache("open_app", 60*time.Second)

// InvalidateOpenAppCache removes the cached app metadata used by OpenPlatformAuth.
func InvalidateOpenAppCache(ctx context.Context, appID string) {
	if appID != "" {
		openAppCache.Delete(ctx, appID)
	}
}

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

	// Lookup app by app_id (cache-first)
	var app entity.OpnApps
	cacheKey := appID
	if !openAppCache.GetJSON(r.Context(), cacheKey, &app) || app.Id == 0 {
		err = dao.OpnApps.Ctx(r.Context()).Where("app_id", appID).Scan(&app)
		if err != nil || app.Id == 0 {
			response.Error(r, consts.ErrUnauthorized)
			return
		}
		openAppCache.Set(r.Context(), cacheKey, &app)
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

	// Per-app rate limiting (fixed window, 1 minute)
	if app.RateLimit > 0 {
		if !checkOpenAppRateLimit(r, app.Id, app.RateLimit) {
			return
		}
	}

	// Get app secret from Redis
	secret, err := getOpenAppSecret(r.Context(), app.Id)
	if err != nil || secret == "" {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// Read request body for signature verification
	// Use r.GetBody() instead of io.ReadAll(r.Body) to preserve body for downstream handlers
	bodyBytes := r.GetBody()

	// Verify HMAC signature (includes body hash for request integrity)
	expectedSig := computeHMACSignature(secret, timestampStr, nonce, r.Method, r.URL.Path, bodyBytes)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		response.Error(r, consts.ErrUnauthorized)
		return
	}

	// 防重放：签名校验通过后，将 nonce 存入 Redis 去重。放在签名校验之后，
	// 避免未认证攻击者用伪造 nonce 刷爆 Redis。命中已存在的 nonce 即判为重放，拒绝。
	if !checkOpenAppNonce(r, app.Id, nonce) {
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

// checkOpenAppNonce enforces replay protection by recording each nonce in Redis.
// 用 SET NX 原子写入 nonce：首次出现返回 OK 放行；已存在说明是重放，写 401 并返回 false。
// Redis 故障时 fail-open（与限流一致），仅记告警，因为时间戳窗口本身已把可重放时间
// 限制在数分钟内。
func checkOpenAppNonce(r *ghttp.Request, appDBID int64, nonce string) bool {
	ctx := r.Context()
	isNew, err := recordOpenNonce(ctx, appDBID, nonce)
	if err != nil {
		g.Log().Warningf(ctx, "[OpenAuth] nonce SET NX failed app=%d: %v", appDBID, err)
		// Fail open on Redis error
		return true
	}
	if !isNew {
		// nonce 已存在 → 重放请求
		response.Error(r, consts.ErrUnauthorized)
		return false
	}
	return true
}

// recordOpenNonce 原子记录 nonce，返回 (isNew, err)。
// isNew==true 表示该 nonce 首次出现（应放行）；false 表示已存在（重放）。
func recordOpenNonce(ctx context.Context, appDBID int64, nonce string) (bool, error) {
	res, err := g.Redis().Do(ctx, "SET", openNonceKey(appDBID, nonce), "1", "NX", "EX", openNonceTTLSeconds())
	if err != nil {
		return false, err
	}
	return !res.IsNil(), nil
}

// openNonceKey 构造按应用隔离的 nonce 去重键。
func openNonceKey(appDBID int64, nonce string) string {
	return fmt.Sprintf("open:nonce:%d:%s", appDBID, nonce)
}

// openNonceTTLSeconds 返回 nonce 记录的存活秒数。
// 取 2 倍时间戳偏移窗口——请求在 [ts-skew, ts+skew] 内都可能通过时间戳校验，
// 记住这么久即可覆盖同一 nonce 的全部可重放区间。
func openNonceTTLSeconds() int {
	return int(2 * openPlatformMaxSkew / time.Second)
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
	encKey := getEncKey(ctx)
	if err == nil && !val.IsNil() && !val.IsEmpty() {
		return crypto.DecryptString(encKey, val.String())
	}
	if err != nil {
		g.Log().Warningf(ctx, "[OpenAuth] Redis secret lookup failed app=%d: %v", appID, err)
	}

	type secretRow struct {
		EncryptedSecret string `json:"encrypted_secret"`
	}
	var row *secretRow
	err = dao.OpnApps.Ctx(ctx).
		Where("id", appID).
		Fields("encrypted_secret").
		Scan(&row)
	if err != nil {
		return "", err
	}
	if row == nil || row.EncryptedSecret == "" {
		return "", fmt.Errorf("secret not persisted; reset app secret required")
	}
	secret, err := crypto.DecryptString(encKey, row.EncryptedSecret)
	if err != nil {
		return "", err
	}
	_, _ = g.Redis().Do(ctx, "SET", fmt.Sprintf("open:secret:%d", appID), row.EncryptedSecret, "EX", 30*86400)
	return secret, nil
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

// checkOpenAppRateLimit enforces per-app rate limiting using a Redis fixed-window counter.
// Returns true if the request is allowed, false if rate limited (429 response already written).
func checkOpenAppRateLimit(r *ghttp.Request, appDBID int64, rateLimit int) bool {
	ctx := r.Context()
	now := time.Now()
	minuteTimestamp := now.Unix() / 60
	key := fmt.Sprintf("ratelimit:open:%d:%d", appDBID, minuteTimestamp)

	count, err := g.Redis().Do(ctx, "INCR", key)
	if err != nil {
		g.Log().Warningf(ctx, "[OpenRateLimit] INCR failed app=%d: %v", appDBID, err)
		// Fail open on Redis error
		return true
	}

	currentCount := count.Int64()
	if currentCount == 1 {
		// First request in this window, set TTL
		_, _ = g.Redis().Do(ctx, "EXPIRE", key, 60)
	}

	remaining := rateLimit - int(currentCount)
	if remaining < 0 {
		remaining = 0
	}
	r.Response.Header().Set("X-RateLimit-Limit", strconv.Itoa(rateLimit))
	r.Response.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

	if int(currentCount) > rateLimit {
		response.ErrorWithCode(r, 429, consts.CodeRateLimitExceeded, consts.MsgRateLimitExceeded)
		return false
	}

	return true
}

// GetOpenPermissions extracts the permissions string (JSON) injected by OpenPlatformAuth.
func GetOpenPermissions(ctx context.Context) string {
	val := ctx.Value(ctxKeyOpenPermissions)
	if val != nil {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// CheckOpenPermission checks if the authenticated app has the given permission.
// Returns error if permission is missing.
func CheckOpenPermission(ctx context.Context, permission string) error {
	perms := GetOpenPermissions(ctx)
	if perms == "" {
		return gerror.NewCode(gcode.New(consts.CodeForbidden, "无权限", nil), "应用无此操作权限")
	}
	var list []string
	if err := json.Unmarshal([]byte(perms), &list); err != nil {
		return gerror.NewCode(gcode.New(consts.CodeForbidden, "无权限", nil), "应用无此操作权限")
	}
	for _, p := range list {
		if p == permission {
			return nil
		}
	}
	return gerror.NewCode(gcode.New(consts.CodeForbidden, "无权限", nil), "应用无此操作权限")
}
