package monitor

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"
)

// GetChannelErrorCount 返回最近5分钟的渠道错误总数
func GetChannelErrorCount(ctx context.Context) (float64, error) {
	record, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT COUNT(*) as cnt FROM chn_error_events WHERE created_at >= NOW() - INTERVAL '5 minutes'")
	if err != nil {
		return 0, err
	}
	if len(record) > 0 {
		return float64(record[0]["cnt"].Int()), nil
	}
	return 0, nil
}

// GetChannelRateLimitCount 返回最近5分钟的限速错误数
func GetChannelRateLimitCount(ctx context.Context) (float64, error) {
	record, err := g.DB().Ctx(ctx).Query(ctx,
		"SELECT COUNT(*) as cnt FROM chn_error_events WHERE error_category = 'rate_limit' AND created_at >= NOW() - INTERVAL '5 minutes'")
	if err != nil {
		return 0, err
	}
	if len(record) > 0 {
		return float64(record[0]["cnt"].Int()), nil
	}
	return 0, nil
}
