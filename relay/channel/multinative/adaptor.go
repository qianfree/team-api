package multinative

import (
	"context"
	"io"
	"net/http"

	"github.com/qianfree/team-api/relay/channel/claude"
	"github.com/qianfree/team-api/relay/channel/gemini"
	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

const ChannelName = "MultiNative"

// Adaptor 多协议原生透传适配器。
// 用于 New API / Sub2API 等同时原生支持 OpenAI / Claude / Gemini 协议的上游网关（自引用场景）。
//
// 与 OpenAI 纯透传适配器不同：后者会把 Claude/Gemini 入站请求归一化为 OpenAI 再转发；
// 本适配器按客户端入站格式委托给对应的原生适配器，请求体按原生协议转发、响应按原生协议解析，
// 保留原生协议保真度（例如 Claude /v1/messages 原样送达上游 /v1/messages）。
type Adaptor struct {
	openaiAdaptor *openai.Adaptor
	claudeAdaptor *claude.Adaptor
	geminiAdaptor *gemini.Adaptor
}

// Init 初始化适配器，预初始化三个原生子适配器。
func (a *Adaptor) Init(info *common.RelayInfo) {
	a.openaiAdaptor = &openai.Adaptor{}
	a.openaiAdaptor.Init(info)

	a.claudeAdaptor = &claude.Adaptor{}
	a.claudeAdaptor.Init(info)

	a.geminiAdaptor = &gemini.Adaptor{}
	a.geminiAdaptor.Init(info)
}

// selectAdaptor 按客户端原始请求格式选择对应的原生适配器。
//   - claude   → Claude 适配器（/v1/messages，x-api-key + anthropic-version）
//   - gemini   → Gemini 适配器（/v1beta/models/{model}:generateContent，x-goog-api-key）
//   - 其余格式 → OpenAI 适配器（openai / responses / 未知格式兜底）
func (a *Adaptor) selectAdaptor(info *common.RelayInfo) common.Adaptor {
	switch info.GetOriginalClientFormat() {
	case constant.RelayFormatClaude:
		return a.claudeAdaptor
	case constant.RelayFormatGemini:
		return a.geminiAdaptor
	default:
		return a.openaiAdaptor
	}
}

// GetRequestURL 构建上游请求 URL（委托给原生适配器，得到原生协议端点）。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	return a.selectAdaptor(info).GetRequestURL(info)
}

// SetupRequestHeader 设置上游请求头（委托给原生适配器，得到原生鉴权头）。
func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	return a.selectAdaptor(info).SetupRequestHeader(header, info)
}

// ConvertRequest 转换请求体（委托给原生适配器）。
// canPassThrough 命中时本方法不会被调用；未命中时（如 responses 入站，或配置了模型映射/参数改写），
// 由匹配的原生适配器处理——同格式入站为原样透传，需映射时替换模型名。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	return a.selectAdaptor(info).ConvertRequest(ctx, info, requestBody)
}

// DoRequest 发送请求到上游（委托给原生适配器）。
func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return a.selectAdaptor(info).DoRequest(ctx, info, requestBody)
}

// DoResponse 处理上游响应（委托给原生适配器，按客户端格式做原生解析或转换）。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	return a.selectAdaptor(info).DoResponse(ctx, resp, info, writer)
}

// GetChannelName 返回渠道名称（用于日志）。
func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

// 确保接口实现
var _ common.Adaptor = (*Adaptor)(nil)
