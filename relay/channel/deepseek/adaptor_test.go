package deepseek

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	// blank import 触发内置转换器注册（relaykit 桥接为唯一路径，测试二进制须自备注册）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
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

// TestConvertRequest_ResponsesChatFallback chat-only 上游（supports_responses=false）：
// responses 入站经共享 ConvertToOpenAI 转 chat 体，且 DeepSeek 定制后处理照常执行
// ——接管点在 adaptor 层（与 claude/gemini 入站同构），injectStreamOptions /
// injectThinkingParams / 模型映射不再被 handler 层桥接跳过（#12 专项修复体）。
func TestConvertRequest_ResponsesChatFallback(t *testing.T) {
	a := &Adaptor{}

	// 基础路径：r2o 转换 + stream_options 注入（IsStream=true）
	info := deepseekResponsesInfo(false)
	out, err := a.ConvertRequest(context.Background(), info, []byte(deepseekResponsesRequestBody))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	raw, _ := io.ReadAll(out)
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("converted body is not json: %v, body: %s", err, raw)
	}
	if _, ok := m["messages"]; !ok {
		t.Errorf("converted body missing messages（r2o 转换未生效）: %s", raw)
	}
	if _, ok := m["input"]; ok {
		t.Errorf("responses input should be converted to chat messages: %s", raw)
	}
	if _, ok := m["stream_options"]; !ok {
		t.Errorf("stream_options should be injected for chat-only upstream: %s", raw)
	}

	// thinking 注入：-thinking 后缀 → injectThinkingParams 生效
	thinkInfo := deepseekResponsesInfo(false)
	thinkInfo.ThinkingEnabled = true
	raw2, err := io.ReadAll(mustConvert(t, a, thinkInfo))
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(raw2), `"thinking"`) {
		t.Errorf("thinking params should be injected for responses inbound on chat-only upstream: %s", raw2)
	}

	// 有状态哨兵：previous_response_id 经共享桥 → ConvertToOpenAI → adaptor 透传
	statefulInfo := deepseekResponsesInfo(false)
	stateful := `{"model":"deepseek-v4-flash","previous_response_id":"resp_1","input":"hi"}`
	_, err = a.ConvertRequest(context.Background(), statefulInfo, []byte(stateful))
	if err == nil {
		t.Fatal("stateful responses on chat-only upstream should fail")
	}
	if !errors.Is(err, constant.ErrStatefulResponsesUnsupported) {
		t.Errorf("error should wrap ErrStatefulResponsesUnsupported（驱动换渠道）, got: %v", err)
	}
}

// mustConvert 执行 ConvertRequest 并读取全部输出。
func mustConvert(t *testing.T, a *Adaptor, info *common.RelayInfo) io.Reader {
	t.Helper()
	out, err := a.ConvertRequest(context.Background(), info, []byte(deepseekResponsesRequestBody))
	if err != nil {
		t.Fatalf("ConvertRequest error: %v", err)
	}
	return out
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
