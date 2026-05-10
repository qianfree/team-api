package channel

import (
	"github.com/qianfree/team-api/relay/channel/ali"
	"github.com/qianfree/team-api/relay/channel/aws"
	"github.com/qianfree/team-api/relay/channel/baidu_v2"
	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/cloudflare"
	"github.com/qianfree/team-api/relay/channel/codex"
	"github.com/qianfree/team-api/relay/channel/coze"
	"github.com/qianfree/team-api/relay/channel/deepseek"
	"github.com/qianfree/team-api/relay/channel/dify"
	"github.com/qianfree/team-api/relay/channel/gemini"
	"github.com/qianfree/team-api/relay/channel/jimeng"
	"github.com/qianfree/team-api/relay/channel/minimax"
	"github.com/qianfree/team-api/relay/channel/mistral"
	"github.com/qianfree/team-api/relay/channel/moonshot"
	"github.com/qianfree/team-api/relay/channel/ollama"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/channel/siliconflow"
	"github.com/qianfree/team-api/relay/channel/submodel"
	"github.com/qianfree/team-api/relay/channel/tencent"
	"github.com/qianfree/team-api/relay/channel/vertex"
	"github.com/qianfree/team-api/relay/channel/volcengine"
	"github.com/qianfree/team-api/relay/channel/xai"
	"github.com/qianfree/team-api/relay/channel/xunfei"
	"github.com/qianfree/team-api/relay/channel/zhipu"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// GetAdaptor 根据供应商类型创建对应的适配器实例。
func GetAdaptor(providerType int) common.Adaptor {
	switch constant.ProviderType(providerType) {
	case constant.ProviderOpenAI:
		return &openai.Adaptor{}
	case constant.ProviderClaude:
		return &claude.Adaptor{}
	case constant.ProviderGemini:
		return &gemini.Adaptor{}
	case constant.ProviderAli:
		return &ali.Adaptor{}
	case constant.ProviderDeepSeek:
		return &deepseek.Adaptor{}
	case constant.ProviderZhipu:
		return &zhipu.Adaptor{}
	case constant.ProviderMoonshot:
		return &moonshot.Adaptor{}
	case constant.ProviderMistral:
		return &mistral.Adaptor{}
	case constant.ProviderXAI:
		return &xai.Adaptor{}
	case constant.ProviderSiliconFlow:
		return &siliconflow.Adaptor{}
	case constant.ProviderCloudflare:
		return &cloudflare.Adaptor{}
	case constant.ProviderSubmodel:
		return &submodel.Adaptor{}
	case constant.ProviderBaiduV2:
		return &baidu_v2.Adaptor{}
	case constant.ProviderVolcengine:
		return &volcengine.Adaptor{}
	case constant.ProviderMiniMax:
		return &minimax.Adaptor{}
	case constant.ProviderOllama:
		return &ollama.Adaptor{}
	case constant.ProviderVertex:
		return &vertex.Adaptor{}
	case constant.ProviderAWS:
		return &aws.Adaptor{}
	case constant.ProviderAzure:
		return &openai.Adaptor{} // Azure OpenAI 兼容 OpenAI 协议
	case constant.ProviderTencent:
		return &tencent.Adaptor{}
	case constant.ProviderXunfei:
		return &xunfei.Adaptor{}
	case constant.ProviderCoze:
		return &coze.Adaptor{}
	case constant.ProviderDify:
		return &dify.Adaptor{}
	case constant.ProviderJimeng:
		return &jimeng.Adaptor{}
	case constant.ProviderCodex:
		return &codex.Adaptor{}
	// OpenAI 兼容供应商（纯透传）
	case constant.ProviderAI360,
		constant.ProviderLingyi,
		constant.ProviderOpenRouter,
		constant.ProviderXInference:
		return &openai.Adaptor{}
	default:
		return nil
	}
}
