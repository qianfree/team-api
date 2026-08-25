package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/qianfree/team-api/relay/channel/openai"
	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/override"
)

// codexCredentials 解析 ApiKey JSON 中的凭证信息
type codexCredentials struct {
	AccessToken string `json:"access_token"`
	AccountID   string `json:"account_id"`
}

// Adaptor Codex 供应商适配器（直连 chatgpt.com 官方 /backend-api/codex/responses 端点）
type Adaptor struct {
	info  *common.RelayInfo
	creds codexCredentials
}

func (a *Adaptor) Init(info *common.RelayInfo) {
	a.info = info
	// ApiKey 格式为 JSON: {"access_token": "...", "account_id": "..."}
	_ = json.Unmarshal([]byte(info.ChannelMeta.ApiKey), &a.creds)
}

// GetRequestURL 构建上游请求 URL。Codex 直连 chatgpt.com 官方 /backend-api/codex/responses 端点；
// compact（长会话上下文压缩）走 /backend-api/codex/responses/compact——对齐 codex CLI
// CompactClient 的相对路径 "responses/compact"（非流式 POST，codex-rs codex-api/src/endpoint/compact.rs）。
func (a *Adaptor) GetRequestURL(info *common.RelayInfo) (string, error) {
	baseURL := strings.TrimSuffix(info.ChannelMeta.BaseURL, "/")

	switch constant.RelayMode(info.RelayMode) {
	case constant.RelayModeResponses:
		return baseURL + "/backend-api/codex/responses", nil
	case constant.RelayModeResponsesCompact:
		return baseURL + "/backend-api/codex/responses/compact", nil
	default:
		return "", fmt.Errorf("codex only supports Responses modes (responses/responses/compact), got relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(header http.Header, info *common.RelayInfo) error {
	header.Set("Authorization", "Bearer "+a.creds.AccessToken)
	header.Set("chatgpt-account-id", a.creds.AccountID)
	// 官方后端必需的伪装头（对齐 Codex CLI 行为）
	header.Set("originator", "codex_cli_rs")
	header.Set("OpenAI-Beta", "responses=experimental")
	// chatgpt.com/backend-api/codex/responses 对 Content-Type 极其严格：
	// 客户端可能省略或带 charset 参数（如 application/json; charset=utf-8）会被上游拒绝，强制精确 media type。
	header.Set("Content-Type", "application/json")
	if info.IsStream {
		header.Set("Accept", "text/event-stream")
	} else {
		header.Set("Accept", "application/json")
	}
	return nil
}

// ConvertRequest 转换请求体。Codex 支持 Responses 主端点与 compact（上下文压缩）两种模式。
// 做模型名映射，并对齐 Codex CLI 剥离官方不兼容字段（store/max_output_tokens/temperature）、补默认 instructions。
// 以 map 形式操作，保留 Responses 其余字段（input/instructions/previous_response_id 等）原样透传，
// 避免结构体 round-trip 丢字段。
func (a *Adaptor) ConvertRequest(ctx context.Context, info *common.RelayInfo, requestBody []byte) (io.Reader, error) {
	// 非 OpenAI 格式先转换为 OpenAI。Responses 入站除外：codex 上游只说 Responses 协议
	//（GetRequestURL 恒为 Responses 专用端点），Responses 体须原样进入下方字段手术——
	// 若错转成 chat 体再发往 Responses 端点会被上游拒绝并触发重试风暴。
	// claude/gemini 入站维持转换（随后被下方模式检查拒绝，不会发出请求）。
	if info.InboundFormat != "" && info.InboundFormat != constant.RelayFormatOpenAI &&
		info.InboundFormat != constant.RelayFormatResponses {
		converted, err := openai.ConvertToOpenAI(requestBody, info)
		if err != nil {
			return nil, err
		}
		requestBody = converted
	}

	// 模式守卫：responses 与 responses/compact 共用同一套字段手术（compact 体仅
	// model/input/instructions/previous_response_id，手术规则对其同样安全）
	if mode := constant.RelayMode(info.RelayMode); mode != constant.RelayModeResponses &&
		mode != constant.RelayModeResponsesCompact {
		return nil, fmt.Errorf("codex only supports Responses modes (responses/responses/compact), got relay mode: %d", info.RelayMode)
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawMap); err != nil {
		// 非 JSON 请求体原样透传（Responses 模式下不应出现，保守处理）
		return bytes.NewReader(requestBody), nil
	}
	if rawMap == nil {
		rawMap = make(map[string]json.RawMessage)
	}

	// 模型名映射
	if info.ChannelMeta.IsModelMapped {
		rawMap["model"] = json.RawMessage(`"` + info.ChannelMeta.UpstreamModelName + `"`)
	}

	// 官方兼容性字段处理（对齐 Codex CLI 行为）：
	//   - store 必须为 false（chatgpt.com 官方后端硬性要求）
	//   - 移除 max_output_tokens / temperature（上游不支持，透传会被拒）
	//   - 移除 frequency_penalty / presence_penalty（非官方 Responses 参数，防御性剥离）
	rawMap["store"] = json.RawMessage("false")
	delete(rawMap, "max_output_tokens")
	delete(rawMap, "temperature")
	delete(rawMap, "frequency_penalty")
	delete(rawMap, "presence_penalty")
	// instructions 字段必须存在，缺省时默认空串
	if _, ok := rawMap["instructions"]; !ok {
		rawMap["instructions"] = json.RawMessage(`""`)
	}

	converted, err := json.Marshal(rawMap)
	if err != nil {
		return nil, fmt.Errorf("marshal converted request failed: %w", err)
	}
	return bytes.NewReader(converted), nil
}

func (a *Adaptor) DoRequest(ctx context.Context, info *common.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	reqURL, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	if err := a.SetupRequestHeader(httpReq.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}

	if hdrOverrides, hdrErr := override.ApplyHeaderOverride(info); hdrErr == nil && len(hdrOverrides) > 0 {
		override.MergeHeaderOverrides(httpReq.Header, hdrOverrides)
	}

	timeout := info.ChannelMeta.Settings.GetTimeoutSeconds(info.RelayMode)

	client := common.NewPooledClient(timeout, info.ChannelMeta.Settings.UseProxy, info.IsStream)

	return client.Do(httpReq)
}

// DoResponse 处理上游响应。委托 OpenAI 适配器处理。
func (a *Adaptor) DoResponse(ctx context.Context, resp *http.Response, info *common.RelayInfo, writer http.ResponseWriter) (*common.Usage, error) {
	delegate := &openai.Adaptor{}
	delegate.Init(info)
	return delegate.DoResponse(ctx, resp, info, writer)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

var _ common.Adaptor = (*Adaptor)(nil)
