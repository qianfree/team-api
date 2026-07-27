package relay

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const channelLeaseDuration = 90 * time.Second

func channelLeaseKey(channelID int64) string {
	return "relay:channel:leases:v1:" + strconv.FormatInt(channelID, 10)
}

func (p *DataProviderImpl) AcquireChannelSlot(ctx context.Context, channelID int64, maxConcurrency int, requestID string) bool {
	if maxConcurrency <= 0 || !lcommon.Config().GetBool(ctx, "channel_capacity_enabled") {
		return true
	}
	now := time.Now().UnixMilli()
	expiresAt := now + channelLeaseDuration.Milliseconds()
	const script = `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
if redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then return 0 end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[4])
redis.call('PEXPIRE', KEYS[1], ARGV[5])
return 1`
	result, err := g.Redis().Do(ctx, "EVAL", script, 1, channelLeaseKey(channelID), now, expiresAt, maxConcurrency, requestID, channelLeaseDuration.Milliseconds()+60000)
	if err != nil {
		g.Log().Warningf(ctx, "[ChannelCapacity] acquire failed open: channel=%d err=%v", channelID, err)
		return true
	}
	return result.Int() == 1
}

func (p *DataProviderImpl) RefreshChannelSlot(ctx context.Context, channelID int64, requestID string) {
	expiresAt := time.Now().Add(channelLeaseDuration).UnixMilli()
	const script = `
if redis.call('ZSCORE', KEYS[1], ARGV[1]) then
  redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
  redis.call('PEXPIRE', KEYS[1], ARGV[3])
  return 1
end
return 0`
	_, _ = g.Redis().Do(ctx, "EVAL", script, 1, channelLeaseKey(channelID), requestID, expiresAt, channelLeaseDuration.Milliseconds()+60000)
}

func (p *DataProviderImpl) ReleaseChannelSlot(ctx context.Context, channelID int64, requestID string) {
	_, _ = g.Redis().Do(ctx, "ZREM", channelLeaseKey(channelID), requestID)
}
