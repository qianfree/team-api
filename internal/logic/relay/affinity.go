package relay

import (
	"context"
	"github.com/qianfree/team-api/internal/dao"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// UpsertAffinity 更新亲和性记录（成功后调用）
func UpsertAffinity(ctx context.Context, tenantID, userID int64, modelName string, channelID int64) {
	now := time.Now()
	expiresAt := now.Add(1800 * time.Second)
	g.DB().Exec(ctx,
		`INSERT INTO chn_channel_affinities (tenant_id, user_id, model_name, channel_id, hit_count, expires_at, updated_at)
		 VALUES (?, ?, ?, ?, 1, ?, ?)
		 ON CONFLICT (tenant_id, user_id, model_name)
		 DO UPDATE SET channel_id = ?, hit_count = chn_channel_affinities.hit_count + 1, expires_at = ?, updated_at = ?`,
		tenantID, userID, modelName, channelID, expiresAt, now, channelID, expiresAt, now)
}

// GetAffinity 获取亲和性渠道
func GetAffinity(ctx context.Context, tenantID, userID int64, modelName string) (int64, bool) {
	var result *struct {
		ChannelID int64 `json:"channel_id"`
	}
	err := dao.ChnChannelAffinities.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("model_name", modelName).
		Where("expires_at > ?", time.Now()).
		Fields("channel_id").
		Scan(&result)
	if err != nil || result == nil {
		return 0, false
	}
	return result.ChannelID, true
}

// DeleteAffinity 删除亲和性记录
func DeleteAffinity(ctx context.Context, tenantID, userID int64, modelName string) {
	dao.ChnChannelAffinities.Ctx(ctx).
		Where("tenant_id", tenantID).
		Where("user_id", userID).
		Where("model_name", modelName).
		Delete()
}

// DeleteAffinityByChannel 删除某渠道的所有亲和性记录
func DeleteAffinityByChannel(ctx context.Context, channelID int64) {
	dao.ChnChannelAffinities.Ctx(ctx).
		Where("channel_id", channelID).
		Delete()
}
