package tencent

import (
	"strings"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
)

const tokenHubBaseURL = "https://tokenhub.tencentmaas.com"

// DispatchAdaptor 腾讯渠道派发适配器：按密钥格式分流。
//   - 含 "|" 的 "secretId|secretKey" → 混元原生 TC3 签名（Adaptor）
//   - 单段 TokenHub Key → OpenAI 兼容协议（tokenhub.tencentmaas.com）
//
// 匿名嵌入 common.Adaptor 接口字段，Init 选定子适配器后其余接口方法自动转发，
// 对 registry / 调度器 / 计费层完全透明。
// 参考 new-api PR #6232。
type DispatchAdaptor struct {
	common.Adaptor
}

// Init 按密钥格式选定子适配器并完成初始化。
func (a *DispatchAdaptor) Init(info *common.RelayInfo) {
	if strings.Contains(info.ChannelMeta.ApiKey, "|") {
		// 三段式 secretId|secretKey → 混元 TC3 签名
		a.Adaptor = &Adaptor{}
	} else {
		// 单段 TokenHub Key → OpenAI 兼容协议
		a.Adaptor = &openai.Adaptor{}
		// 仅当未显式自定义 base URL（空或为混元默认）时改写为 TokenHub，
		// 用户显式配置的代理网关地址予以保留。
		if isHunyuanOrDefault(info.ChannelMeta.BaseURL) {
			info.ChannelMeta.BaseURL = tokenHubBaseURL
		}
	}
	a.Adaptor.Init(info)
}

// GetChannelName 覆盖子适配器名称，日志统一记为腾讯渠道。
func (a *DispatchAdaptor) GetChannelName() string {
	return ChannelName
}

// isHunyuanOrDefault 判断 base URL 是否为空或指向混元默认地址。
// 兼容带/不带尾斜杠、http/https。
func isHunyuanOrDefault(baseURL string) bool {
	if baseURL == "" {
		return true
	}
	trimmed := strings.TrimPrefix(strings.TrimPrefix(baseURL, "https://"), "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	return trimmed == "hunyuan.tencentcloudapi.com"
}
