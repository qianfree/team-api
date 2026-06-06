package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"

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
		Settings  string `json:"settings"`
	}

	var ch *channelRow
	err := dao.ChnChannels.Ctx(ctx).
		Where("id", channelID).
		Fields("id, name, type, base_url, test_model, status, settings").
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

	// 解析渠道设置中的 use_proxy
	useProxy := false
	if ch.Settings != "" {
		var settings struct {
			UseProxy bool `json:"use_proxy"`
		}
		if json.Unmarshal([]byte(ch.Settings), &settings) == nil {
			useProxy = settings.UseProxy
		}
	}

	// 发送最小测试请求
	startTime := time.Now()
	result := sendTestRequest(ctx, ch.Type, ch.BaseURL, apiKey, testModel, useProxy)
	latencyMs := time.Since(startTime).Milliseconds()

	// 更新健康度
	if result.Success {
		relay.UpdateHealthScoreDirect(ctx, channelID, true, float64(latencyMs))
	} else {
		relay.UpdateHealthScoreDirect(ctx, channelID, false, float64(latencyMs))
	}

	// 记录测试结果日志
	if result.Success {
		g.Log().Infof(ctx, "[ChannelTest] 渠道 %s (%d) 测试成功 | 模型: %s | 延迟: %dms | 代理: %v",
			ch.Name, channelID, testModel, latencyMs, useProxy)
	} else {
		g.Log().Warningf(ctx, "[ChannelTest] 渠道 %s (%d) 测试失败 | 模型: %s | 延迟: %dms | 代理: %v | 错误: %s",
			ch.Name, channelID, testModel, latencyMs, useProxy, result.Error)
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
