package admin

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"github.com/qianfree/team-api/api/admin/v1"
	"github.com/qianfree/team-api/internal/consts"
	"github.com/qianfree/team-api/internal/dao"
	"github.com/qianfree/team-api/internal/dispatchadapter"
	"github.com/qianfree/team-api/internal/logic/common"
	"github.com/qianfree/team-api/internal/logic/relay"
)

// TestChannel 测试渠道可用性（发送最小请求验证）。
// 不限制渠道状态：禁用中的渠道也允许手动测试，便于管理员在启用放流量前确认已恢复；
// 自动探测 cron 只查 active 渠道，禁用渠道不会被自动探测。
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

	testModel := req.ModelName
	if testModel == "" {
		testModel = ch.TestModel
	}
	if testModel == "" {
		return nil, common.NewBadRequestError("请指定测试模型名")
	}

	// 将系统内模型名称转换为该渠道配置的上游模型名称
	// chn_abilities 表存储了每个渠道每个模型的上游模型映射
	// 例如：平台标准名 "deepseek-v4-flash" → 上游实际名 "DeepSeek-V4-Flash"
	type abilityRow struct {
		UpstreamModel string `json:"upstream_model"`
	}
	var abilityInfo abilityRow
	upstreamModel := testModel

	// 查询该渠道配置的上游模型名称
	err = dao.ChnAbilities.Ctx(ctx).
		Where("channel_id", channelID).
		Where("model_name", testModel).
		Where("enabled", true).
		Fields("upstream_model").
		Limit(1).
		Scan(&abilityInfo)

	if abilityInfo.UpstreamModel != "" {
		upstreamModel = abilityInfo.UpstreamModel
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
		return nil, common.NewBusinessError(consts.CodeNotFound, "渠道没有可用的 API Key")
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
	g.Log().Infof(ctx, "[ChannelProbe] 渠道 %s (%d) 开始探测 | 模型: %s (上游: %s) | 渠道状态: %s | 代理: %v",
		ch.Name, channelID, testModel, upstreamModel, ch.Status, useProxy)
	startTime := time.Now()
	result := sendTestRequest(ctx, ch.Type, ch.BaseURL, apiKey, upstreamModel, useProxy)
	latencyMs := time.Since(startTime).Milliseconds()

	// 更新健康度（喂给调度引擎的健康体系；探测失败按瞬时错误轻罚）。
	// 仅 active 渠道上报：禁用/测试中渠道不在调度目录，喂健康分只会无意义拉低展示值；
	// 渠道启用时 MarkChannelRecovered 会复位熔断，无需在禁用期间预热。
	// model 必须传平台标准名（testModel）而非上游名：调度的健康/熔断 Redis key 统一
	// 以平台名为口径（真实流量报 profile.Model，维护快照按 catalog 的 model_name 读取），
	// 传上游名会写到无人读取的 key——探测既影响不了健康分，也关不掉 HALF_OPEN 熔断。
	if ch.Status == "active" {
		dispatchadapter.ReportProbeOutcome(ctx, channelID, testModel, result.Success, float64(latencyMs))
		if dispatchadapter.CatalogHasModel(channelID, testModel) {
			g.Log().Infof(ctx, "[ChannelProbe] 渠道 %s (%d) 探测结果已投递健康上报 | 模型: %s | success: %v | 延迟: %dms（EWMA 变化将打印 [ChannelHealth] 日志，5 分钟内落盘刷新）",
				ch.Name, channelID, testModel, result.Success, latencyMs)
		} else {
			// 探测目标不在调度目录：EWMA 照常写入 Redis，但调度不读、维护快照不落盘，
			// 管理后台健康分不会变化——探测看起来"生效了但没效果"的常见根因
			g.Log().Warningf(ctx, "[ChannelProbe] 渠道 %s (%d) 探测结果已投递健康上报，但模型 %s 不在调度目录（渠道无启用的模型能力行或该模型未配置能力）| 健康分不会落盘展示，渠道/模型不参与调度，请检查渠道的模型能力配置",
				ch.Name, channelID, testModel)
		}
	} else {
		g.Log().Infof(ctx, "[ChannelProbe] 渠道 %s (%d) 状态为 %s（非 active），探测结果不上报健康体系",
			ch.Name, channelID, ch.Status)
	}

	// 记录测试结果日志
	if result.Success {
		g.Log().Infof(ctx, "[ChannelProbe] 渠道 %s (%d) 探测成功 | 模型: %s (上游: %s) | 延迟: %dms | 代理: %v",
			ch.Name, channelID, testModel, upstreamModel, latencyMs, useProxy)
	} else {
		g.Log().Warningf(ctx, "[ChannelProbe] 渠道 %s (%d) 探测失败 | 模型: %s (上游: %s) | 延迟: %dms | 代理: %v | 错误: %s",
			ch.Name, channelID, testModel, upstreamModel, latencyMs, useProxy, result.Error)
	}

	return &v1.ChannelTestRes{
		Success:   result.Success,
		Latency:   latencyMs,
		ModelName: upstreamModel,
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
