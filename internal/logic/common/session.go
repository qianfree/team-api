package common

import (
	"context"
	"fmt"
	do "github.com/qianfree/team-api/internal/model/do"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"

	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/model/entity"
)

// SessionInfo represents a user session for API responses.
type SessionInfo struct {
	ID         int64       `json:"id"`
	Jti        string      `json:"jti"`
	UserType   string      `json:"user_type"`
	UserID     int64       `json:"user_id"`
	TenantID   int64       `json:"tenant_id,omitempty"`
	DeviceInfo string      `json:"device_info,omitempty"`
	IpAddress  string      `json:"ip_address"`
	ExpiresAt  *gtime.Time `json:"expires_at"`
	CreatedAt  *gtime.Time `json:"created_at"`
}

// CreateSession creates a new session in the database and enforces max session limit.
func CreateSession(ctx context.Context, userType string, userID, tenantID int64, refreshTokenHash, ipAddress, deviceInfo, jti string) (sessionID int64, err error) {
	maxSessions := GetMaxSessions(ctx, userType)
	refreshExpire := getRefreshExpire(ctx)
	expiresAt := gtime.New(time.Now().Add(refreshExpire))

	// 事务采用 ctx 传播式写法：闭包内统一使用 dao.Xxx.Ctx(ctx)，事务由 ctx 自动挂载。
	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Count existing active sessions
		count, err := dao.SysSessions.Ctx(ctx).
			Where("user_type", userType).
			Where("user_id", userID).
			Where("expires_at > NOW()").
			Count()
		if err != nil {
			return gerror.Wrapf(err, "count sessions")
		}

		// Enforce max sessions: delete oldest sessions if over limit
		if int(count) >= maxSessions {
			overCount := int(count) - maxSessions + 1
			// PostgreSQL does not support DELETE ... ORDER BY ... LIMIT,
			// so use a subquery to find the oldest session IDs first.
			var oldIDs []int64
			err = dao.SysSessions.Ctx(ctx).
				Fields("id").
				Where("user_type", userType).
				Where("user_id", userID).
				Where("expires_at > NOW()").
				OrderAsc("created_at").
				Limit(overCount).
				Scan(&oldIDs)
			if err != nil {
				return gerror.Wrapf(err, "find old sessions")
			}
			if len(oldIDs) > 0 {
				_, err = dao.SysSessions.Ctx(ctx).
					WhereIn("id", oldIDs).
					Delete()
				if err != nil {
					return gerror.Wrapf(err, "evict old sessions")
				}
			}
		}

		// Insert new session
		result, err := dao.SysSessions.Ctx(ctx).Data(do.SysSessions{
			UserType:         userType,
			UserId:           userID,
			TenantId:         tenantID,
			RefreshTokenHash: refreshTokenHash,
			IpAddress:        ipAddress,
			DeviceInfo:       deviceInfo,
			Jti:              jti,
			ExpiresAt:        expiresAt,
		}).Insert()
		if err != nil {
			return gerror.Wrapf(err, "insert session")
		}

		id, err := result.LastInsertId()
		if err != nil {
			return gerror.Wrapf(err, "get session id")
		}
		sessionID = id
		return nil
	})

	return sessionID, err
}

// RevokeSession revokes a single session by ID (DB only; use MarkSessionRevoked for Redis).
// userType 限定会话所属用户体系（admin/tenant），防止跨控制台按 ID 吊销他人会话。
func RevokeSession(ctx context.Context, userType string, sessionID int64) error {
	_, err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
		Where("user_type", userType).
		Delete()
	return err
}

// RevokeAllSessions revokes all active sessions for a user (DB + Redis).
func RevokeAllSessions(ctx context.Context, userType string, userID int64) error {
	// Mark all active sessions as revoked in Redis first
	sessions, err := ListSessions(ctx, userType, userID)
	if err != nil {
		return err
	}
	for _, sess := range sessions {
		MarkSessionRevoked(ctx, sess.Jti)
	}

	// Delete from DB
	_, err = dao.SysSessions.Ctx(ctx).
		Where("user_type", userType).
		Where("user_id", userID).
		Where("expires_at > NOW()").
		Delete()
	return err
}

// ListSessions returns all active sessions for a user.
func ListSessions(ctx context.Context, userType string, userID int64) ([]SessionInfo, error) {
	var sessions []entity.SysSessions
	err := dao.SysSessions.Ctx(ctx).
		Where("user_type", userType).
		Where("user_id", userID).
		Where("expires_at > NOW()").
		OrderDesc("created_at").
		Scan(&sessions)
	if err != nil {
		return nil, err
	}

	result := make([]SessionInfo, len(sessions))
	for i, s := range sessions {
		result[i] = SessionInfo{
			ID:         s.Id,
			Jti:        s.Jti,
			UserType:   s.UserType,
			UserID:     s.UserId,
			TenantID:   s.TenantId,
			DeviceInfo: s.DeviceInfo,
			IpAddress:  s.IpAddress,
			ExpiresAt:  s.ExpiresAt,
			CreatedAt:  s.CreatedAt,
		}
	}
	return result, nil
}

// IsSessionRevoked checks if a session has been revoked (via Redis blacklist).
// Uses the JWT ID (jti) as the cache key — unique per session, independent of DB sequences.
func IsSessionRevoked(ctx context.Context, jti string) bool {
	key := fmt.Sprintf("session:revoked:%s", jti)
	val, err := g.Redis().Do(ctx, "GET", key)
	if err != nil || val.IsNil() {
		return false
	}
	return val.Bool()
}

// MarkSessionRevoked adds a session to the Redis blacklist for instant revocation.
func MarkSessionRevoked(ctx context.Context, jti string) {
	key := fmt.Sprintf("session:revoked:%s", jti)
	// Set TTL to refresh token expiry (7 days) to auto-cleanup
	_, _ = g.Redis().Do(ctx, "SETEX", key, 7*24*3600, "1")
}

// GetSessionByID retrieves a session by its ID.
// userType 限定会话所属用户体系（admin/tenant），防止跨控制台按 ID 读取他人会话。
func GetSessionByID(ctx context.Context, userType string, sessionID int64) (*entity.SysSessions, error) {
	var session *entity.SysSessions
	err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
		Where("user_type", userType).
		Where("expires_at > NOW()").
		Scan(&session)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return session, nil
}

// GetSessionByRefreshToken 按刷新令牌定位活跃会话，并检测"已轮换令牌重放"。
//
// 新格式令牌内嵌会话 ID（random.sessionID）：按 ID 定位会话后与随机段哈希比对，
// 命中即正常返回；会话仍在但哈希不匹配 = 该令牌已被轮换过 = 重放（典型场景：
// 令牌被盗、攻击者抢先刷新，受害者手里成了旧令牌）。重放直接吊销整个会话
// （攻击者手中的新令牌一并作废，迫使用户重新登录）并写登录历史留痕。
// 存量旧格式令牌（无会话 ID）退化为按哈希直查，行为与历史版本一致。
//
// 返回 (session, replayed, err)：replayed=true 时会话已在内部吊销，调用方返回 401 即可。
func GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*entity.SysSessions, bool, error) {
	if randomPart, sessionID, ok := SplitRefreshToken(refreshToken); ok {
		var sess *entity.SysSessions
		err := dao.SysSessions.Ctx(ctx).
			Where("id", sessionID).
			Where("expires_at > NOW()").
			Scan(&sess)
		if err != nil {
			return nil, false, err
		}
		if sess == nil {
			// 会话已过期/被清理，按不存在拒绝，无重放可言
			return nil, false, nil
		}
		if sess.RefreshTokenHash == HashRefreshToken(randomPart) {
			return sess, false, nil
		}

		// 哈希不匹配 ≠ 重放：只有随机段哈希命中上一代令牌（轮换时记入 Redis）
		// 才判定为重放。若仅凭"会话存在但哈希不匹配"就吊销，会话 ID 是顺序
		// bigint，任何人拼 {垃圾串}.{受害者会话ID} 即可恶意吊销他人会话（DoS）。
		if prev, rErr := g.Redis().Do(ctx, "GET", refreshPrevKey(sessionID)); rErr == nil && !prev.IsNil() && prev.String() == HashRefreshToken(randomPart) {
			// 重放：吊销会话（Redis jti 黑名单 + 删会话行），留痕后拒绝
			MarkSessionRevoked(ctx, sess.Jti)
			if _, delErr := dao.SysSessions.Ctx(ctx).Where("id", sessionID).Delete(); delErr != nil {
				g.Log().Errorf(ctx, "revoke replayed session %d: %v", sessionID, delErr)
			}
			ip, ua := "", ""
			if r := g.RequestFromCtx(ctx); r != nil {
				ip = r.GetClientIp()
				ua = r.GetHeader("User-Agent")
			}
			_ = RecordLoginHistory(ctx, sess.UserType, sess.UserId, sess.TenantId, "refresh", ip, ua, "", false, "刷新令牌重放，会话已吊销")
			g.Log().Warningf(ctx, "refresh token replay detected: user_type=%s user_id=%d session=%d", sess.UserType, sess.UserId, sessionID)
			return nil, true, nil
		}

		// 既不是当前令牌也不是上一代：普通无效令牌，拒绝但不吊销
		return nil, false, nil
	}

	// 存量旧格式令牌：按哈希直查（无法定位轮换来源，退化为普通拒绝）
	sess, err := GetSessionByRefreshHash(ctx, HashRefreshTokenForCompare(refreshToken))
	return sess, false, err
}

// refreshPrevKey 是上一代刷新令牌哈希的 Redis key（重放检测用，见 RotateSessionRefreshToken）。
func refreshPrevKey(sessionID int64) string {
	return fmt.Sprintf("session:refresh_prev:%d", sessionID)
}

// RotateSessionRefreshToken 轮换会话刷新令牌：生成新随机段并原子替换库中哈希。
// 返回绑定会话 ID 后的新令牌（返回给客户端的最终形态）。
// UPDATE 同时校验旧哈希，并发下同一旧令牌只有一个请求能成功。
func RotateSessionRefreshToken(ctx context.Context, sessionID int64, oldRefreshToken, ipAddress, deviceInfo string) (string, error) {
	newRandom, err := GenerateRefreshToken()
	if err != nil {
		return "", err
	}
	oldHash := HashRefreshTokenForCompare(oldRefreshToken)

	refreshExpire := getRefreshExpire(ctx)
	expiresAt := gtime.New(time.Now().Add(refreshExpire))

	result, err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
		Where("refresh_token_hash", oldHash).
		Where("expires_at > NOW()").
		Data(do.SysSessions{
			RefreshTokenHash: HashRefreshToken(newRandom),
			IpAddress:        ipAddress,
			DeviceInfo:       deviceInfo,
			ExpiresAt:        expiresAt,
		}).Update()
	if err != nil {
		return "", gerror.Wrapf(err, "update session")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return "", gerror.Wrapf(err, "check rows affected")
	}
	if rowsAffected == 0 {
		return "", NewUnauthorizedError("会话不存在或已过期")
	}

	// 记录上一代令牌哈希（TTL 对齐刷新周期）：重放检测必须能证明"这个随机段
	// 曾经属于该会话"，否则顺序会话 ID 会被用来恶意吊销他人会话。Redis 不可用
	// 时仅损失检测能力（降级为普通拒绝），不产生误吊销。
	if _, rErr := g.Redis().Do(ctx, "SETEX", refreshPrevKey(sessionID), 7*24*3600, oldHash); rErr != nil {
		g.Log().Warningf(ctx, "record prev refresh hash for session %d: %v", sessionID, rErr)
	}

	return BindSessionIDToRefreshToken(newRandom, sessionID), nil
}

// GetSessionByRefreshHash retrieves an active session by refresh token hash.
func GetSessionByRefreshHash(ctx context.Context, refreshTokenHash string) (*entity.SysSessions, error) {
	var session *entity.SysSessions
	err := dao.SysSessions.Ctx(ctx).
		Where("refresh_token_hash", refreshTokenHash).
		Where("expires_at > NOW()").
		Scan(&session)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}
	return session, nil
}

// CleanExpiredSessions removes expired sessions from the database.
func CleanExpiredSessions(ctx context.Context) (int64, error) {
	result, err := dao.SysSessions.Ctx(ctx).
		Where("expires_at < NOW()").
		Delete()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetCtxUserID extracts user ID from context.
func GetCtxUserID(ctx context.Context) int64 {
	val := ctx.Value("userId")
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetCtxUserRole extracts the admin privilege flag ("super_admin" / "admin") from context.
// 注意它是特权标记而非业务角色：业务角色存放在 sys_admin_user_roles，
// 这里只用于判断是否走超级管理员短路。
func GetCtxUserRole(ctx context.Context) string {
	val := ctx.Value("role")
	if val == nil {
		return ""
	}
	if role, ok := val.(string); ok {
		return role
	}
	return ""
}

// GetCtxSessionID extracts session ID from context.
func GetCtxSessionID(ctx context.Context) int64 {
	val := ctx.Value("sessionId")
	if val == nil {
		return 0
	}
	if id, ok := val.(int64); ok {
		return id
	}
	return 0
}

// GetCtxJti extracts the JWT ID (jti) from context.
func GetCtxJti(ctx context.Context) string {
	val := ctx.Value("jti")
	if val == nil {
		return ""
	}
	if jti, ok := val.(string); ok {
		return jti
	}
	return ""
}
