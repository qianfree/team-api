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

	err = g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// Count existing active sessions
		count, err := tx.Model("sys_sessions").Ctx(ctx).
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
			err = tx.Model("sys_sessions").Ctx(ctx).
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
				_, err = tx.Model("sys_sessions").Ctx(ctx).
					WhereIn("id", oldIDs).
					Delete()
				if err != nil {
					return gerror.Wrapf(err, "evict old sessions")
				}
			}
		}

		// Insert new session
		result, err := tx.Model("sys_sessions").Ctx(ctx).Data(do.SysSessions{
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

// RefreshSession rotates a refresh token: invalidates the old one and creates a new session.
func RefreshSession(ctx context.Context, sessionID int64, oldRefreshTokenHash, newRefreshTokenHash, ipAddress, deviceInfo string) error {
	refreshExpire := getRefreshExpire(ctx)
	expiresAt := gtime.New(time.Now().Add(refreshExpire))

	result, err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
		Where("refresh_token_hash", oldRefreshTokenHash).
		Where("expires_at > NOW()").
		Data(do.SysSessions{
			RefreshTokenHash: newRefreshTokenHash,
			IpAddress:        ipAddress,
			DeviceInfo:       deviceInfo,
			ExpiresAt:        expiresAt,
		}).Update()
	if err != nil {
		return gerror.Wrapf(err, "update session")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrapf(err, "check rows affected")
	}
	if rowsAffected == 0 {
		return NewUnauthorizedError("会话不存在或已过期")
	}

	return nil
}

// RevokeSession revokes a single session by ID (DB only; use MarkSessionRevoked for Redis).
func RevokeSession(ctx context.Context, sessionID int64) error {
	_, err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
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
func GetSessionByID(ctx context.Context, sessionID int64) (*entity.SysSessions, error) {
	var session *entity.SysSessions
	err := dao.SysSessions.Ctx(ctx).
		Where("id", sessionID).
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
