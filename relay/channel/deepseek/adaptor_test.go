package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// deepseekResponsesInfo 构造 responses 入站的 DeepSeek 渠道 RelayInfo，
// supportsResponses 控制渠道是否声明上游原生支持 Responses 协议。
func deepseekResponsesInfo(supportsResponses bool) *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(constant.RelayModeResponses),
		IsStream:        true,
		InboundFormat:   constant.RelayFormatResponses,
		ClientFormat:    constant.RelayFormatResponses,
		OriginModelName: "deepseek-v4-flash",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderDeepSeek),
			BaseURL:           "https://api.deepseek.example.com",
			UpstreamModelName: "deepseek-v4-flash",
			IsModelMapped:     false,
			SupportsResponses: supportsResponses,
		},
	}
}

// TestGetRequestURL_ResponsesFlagRouting responses 模式按渠道开关路由：
// 开 → /v1/responses（compact → /v1/responses/compact），关 → /v1/chat/completions 兜底。
func TestGetRequestURL_ResponsesFlagRouting(t *testing.T) {
	cases := []struct {
		name             string
		mode             constant.RelayMode
		supportsResponse bool
		want             string
	}{
		{"responses_on", constant.RelayModeResponses, true, "https://api.deepseek.example.com/v1/responses"},
		{"responses_compact_on", constant.RelayModeResponsesCompact, true, "https://api.deepseek.example.com/v1/responses/compact"},
		{"responses_off", constant.RelayModeResponses, false, "https://api.deepseek.example.com/v1/chat/completions"},
		{"responses_compact_off", constant.RelayModeResponsesCompact, false, "https://api.deepseek.example.com/v1/chat/completions"},
	}
	for _, c := range cases {
		a := &Adaptor{}
		info := deepseekResponsesInfo(c.supportsResponse)
		info.RelayMode = int(c.mode)
		got, err := a.GetRequestURL(info)
		if err != nil {
			t.Fatalf("GetRequestURL(%s) error: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("GetRequestURL(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

const deepseekResponsesRequestBody = `{"model":"deepseek-v4-flash","instructions":"You are helpful.","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"你好"}]}],"stream":true}`

// TestConvertRequest_ResponsesChatFallback_HardFail chat-only 上游的 Responses→chat
// 转换已由 relaykit 接管（handler 侧矩阵 Responses→OpenAI + PostProcessConvertedRequest
// 注入 stream_options/thinking），adaptor.ConvertRequest 不再承载该路径。
// 此处验证守卫语义：该组合到达 adaptor 时（仅矩阵注册缺失的程序性异常）必须显式报错，
// 不得把 Responses 体静默透传给 chat-only 端点。
func TestConvertRequest_ResponsesChatFallback_HardFail(t *testing.T) {
	a := &Adaptor{}
	info := deepseekResponsesInfo(false)
	_, err := a.ConvertRequest(context.Background(), info, []byte(deepseekResponsesRequestBody))
	if err == nil {
		t.Fatal("chat-only 上游的 Responses 入站到达 adaptor 应显式报错，而非静默透传")
	}
	var relayErr *constant.RelayError
	if !errors.As(err, &relayErr) {
		t.Fatalf("want *constant.RelayError, got %T: %v", err, err)
	}
}

// TestPostProcessRequest_StreamOptions OpenAI 同格式入站的 adaptor 路径：
// 流式请求须注入 stream_options（计费需要 usage）。
func TestPostProcessRequest_StreamOptions(t *testing.T) {
	info := deepseekResponsesInfo(false)
	info.IsStream = true
	chatBody := []byte(`{"model":"deepseek-v4-flash","stream":true,"messages":[{"role":"user","content":"你好"}]}`)

	out, err := postProcessRequest(chatBody, info)
	if err != nil {
		t.Fatalf("postProcessRequest error: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("output is not json: %v, body: %s", err, out)
	}
	if _, ok := m["stream_options"]; !ok {
		t.Errorf("stream_options should be injected for stream request: %s", out)
	}
}

// TestConvertRequest_ResponsesUpstreamPassthrough 渠道开启 supports_responses 时，
// 保持 Responses 格式直连（仅模型映射 + reasoning 注入），不得转成 chat 体。
func TestConvertRequest_ResponsesUpstreamPassthrough(t *testing.T) {
	a := &Adaptor{}
	info := deepseekResponsesInfo(true)
	out, err := a.ConvertRequest(context.Background(), info, []byte(deepseekResponsesRequestBody))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("converted body is not json: %v, body: %s", err, raw)
	}
	if _, ok := m["input"]; !ok {
		t.Errorf("input field should be preserved for responses-native upstream: %s", raw)
	}
	if _, ok := m["messages"]; ok {
		t.Errorf("messages field should not appear for responses-native upstream: %s", raw)
	}
}
