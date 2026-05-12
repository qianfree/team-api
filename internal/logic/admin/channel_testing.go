package admin

import (
	"context"
	"time"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
)

// TestChannel 测试渠道可用性（发送最小请求验证）
func (s *sAdmin) TestChannel(ctx context.Context, req *v1.ChannelTestReq) (*v1.ChannelTestRes, error) {
	channelID := req.ID

	// 获取渠道信息
	type channelRow struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Type      int    `json:"type"`
		BaseURL   string `json:"base_url"`
		TestModel string `json:"test_model"`
		Status    string `json:"status"`
	}

	var ch *channelRow
	err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Fields("id, name, type, base_url, test_model, status").
		Scan(&ch)
	if err != nil {
		return nil, err
	}
	if ch == nil {
		return nil, common.NewNotFoundError("渠道")
	}
	if ch.Status == "disabled" {
		return nil, common.NewBadRequestError("渠道已禁用")
	}

	testModel := req.ModelName
	if testModel == "" {
		testModel = ch.TestModel
	}
	if testModel == "" {
		return nil, common.NewBadRequestError("请指定测试模型名")
	}

	// 获取渠道的 API Key
	type keyRow struct {
		EncryptedKey string `json:"encrypted_key"`
	}
	var keyInfo *keyRow
	err = dao.ChnChannelKeys.Ctx(ctx).
		Where("channel_id", channelID).
		Where("status", "active").
		Fields("encrypted_key").
		OrderAsc("last_used_at").
		Limit(1).
		Scan(&keyInfo)
	if err != nil || keyInfo == nil || keyInfo.EncryptedKey == "" {
		return nil, common.NewNotFoundError("渠道没有可用的 API Key")
	}

	encKey := relay.GetEncryptionKey()
	apiKey, err := relay.DecryptChannelKey(encKey, keyInfo.EncryptedKey)
	if err != nil {
		return nil, common.NewBadRequestError("解密 API Key 失败")
	}

	// 发送最小测试请求
	startTime := time.Now()
	result := sendTestRequest(ctx, ch.Type, ch.BaseURL, apiKey, testModel)
	latencyMs := time.Since(startTime).Milliseconds()

	// 更新健康度
	if result.Success {
		relay.UpdateHealthScoreDirect(ctx, channelID, true, float64(latencyMs))
	} else {
		relay.UpdateHealthScoreDirect(ctx, channelID, false, float64(latencyMs))
	}

	return &v1.ChannelTestRes{
		Success:   result.Success,
		Latency:   latencyMs,
		ModelName: testModel,
		Error:     result.Error,
		Request:   result.Request,
		Response:  result.Response,
	}, nil
}

// testResult 测试结果
type testResult struct {
	Success  bool
	Error    string
	Response string
	Request  *v1.ChannelTestReqDetail
}
