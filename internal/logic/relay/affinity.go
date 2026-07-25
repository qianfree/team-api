package relay

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const affinityRedisPrefix = "relay:affinity:v2"

func affinityIdentity(tenantID, userID, apiKeyID int64, modelName string) (redisKey, seed string) {
	seed = fmt.Sprintf("%d:%d:%d:%s", tenantID, userID, apiKeyID, modelName)
	digest := sha256.Sum256([]byte(seed))
	return affinityRedisPrefix + ":key:" + hex.EncodeToString(digest[:]), seed
}

func affinityTTL(ctx context.Context) time.Duration {
	seconds := lcommon.Config().GetInt(ctx, "channel_affinity_ttl_seconds")
	if seconds <= 0 {
		seconds = 1800
	}
	return time.Duration(seconds) * time.Second
}

func affinityEnabled(ctx context.Context) bool {
	return lcommon.Config().GetBool(ctx, "channel_affinity_enabled")
}

func getAffinity(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) (int64, bool) {
	if !affinityEnabled(ctx) {
		return 0, false
	}
	key, _ := affinityIdentity(tenantID, userID, apiKeyID, modelName)
	value, err := g.Redis().Do(ctx, "GET", key)
	if err != nil || value.IsNil() {
		return 0, false
	}
	channelID, err := strconv.ParseInt(value.String(), 10, 64)
	return channelID, err == nil && channelID > 0
}

func setAffinity(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string, channelID int64) {
	if !affinityEnabled(ctx) || channelID <= 0 {
		return
	}
	key, _ := affinityIdentity(tenantID, userID, apiKeyID, modelName)
	ttlSeconds := int64(affinityTTL(ctx).Seconds())
	reversePrefix := affinityRedisPrefix + ":channel:"
	const script = `
local old = redis.call('GET', KEYS[1])
if old and old ~= ARGV[1] then
  redis.call('SREM', ARGV[3] .. old, KEYS[1])
end
redis.call('SETEX', KEYS[1], ARGV[2], ARGV[1])
redis.call('SADD', ARGV[3] .. ARGV[1], KEYS[1])
redis.call('EXPIRE', ARGV[3] .. ARGV[1], ARGV[2] + 60)
return 1`
	if _, err := g.Redis().Do(ctx, "EVAL", script, 1, key, channelID, ttlSeconds, reversePrefix); err != nil {
		g.Log().Warningf(ctx, "[Affinity] set failed: %v", err)
	}
}

func deleteAffinity(ctx context.Context, tenantID, userID, apiKeyID int64, modelName string) {
	key, _ := affinityIdentity(tenantID, userID, apiKeyID, modelName)
	reversePrefix := affinityRedisPrefix + ":channel:"
	const script = `
local old = redis.call('GET', KEYS[1])
if old then redis.call('SREM', ARGV[1] .. old, KEYS[1]) end
return redis.call('DEL', KEYS[1])`
	_, _ = g.Redis().Do(ctx, "EVAL", script, 1, key, reversePrefix)
}

// InvalidateChannelAffinities removes all distributed affinity records bound to a channel.
func InvalidateChannelAffinities(ctx context.Context, channelID int64) {
	reverseKey := affinityRedisPrefix + ":channel:" + strconv.FormatInt(channelID, 10)
	const script = `
local keys = redis.call('SMEMBERS', KEYS[1])
for _, key in ipairs(keys) do redis.call('DEL', key) end
redis.call('DEL', KEYS[1])
return #keys`
	if _, err := g.Redis().Do(ctx, "EVAL", script, 1, reverseKey); err != nil {
		g.Log().Warningf(ctx, "[Affinity] invalidate channel %d failed: %v", channelID, err)
	}
}
