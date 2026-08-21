package deepseek

import (
	"context"
	"encoding/json"
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

// TestConvertRequest_ResponsesChatFallback 收割后契约：responses→chat 转换已上收
// handler 层桥接（relaykit），adaptor 的 chat-only 兜底分支不再做 legacy 转换——
// 走到该分支说明桥接未接管，按转换失败报错。
func TestConvertRequest_ResponsesChatFallback(t *testing.T) {
	a := &Adaptor{}
	info := deepseekResponsesInfo(false)
	_, err := a.ConvertRequest(context.Background(), info, []byte(deepseekResponsesRequestBody))
	if err == nil {
		t.Fatal("expected conversion error after harvest（handler 层桥接未接管时 adaptor 必须报错）")
	}
	if !strings.Contains(err.Error(), "responses→openai") {
		t.Errorf("error should mention responses→openai direction, got: %v", err)
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
