package deepseek

import (
	"context"
	"encoding/json"
	"io"
	"strings"
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

// TestConvertRequest_ResponsesChatFallback 渠道未开 supports_responses 时，
// Responses 请求必须转换为 Chat 格式（instructions→system、input→messages、stream 保留），
// 不能再以 Responses 体直发 chat-only 上游。
func TestConvertRequest_ResponsesChatFallback(t *testing.T) {
	a := &Adaptor{}
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

	// Responses 专属字段必须消失，chat 专属字段必须存在
	if _, ok := m["input"]; ok {
		t.Error("input field should be converted to messages")
	}
	msgsRaw, ok := m["messages"]
	if !ok {
		t.Fatalf("messages field missing: %s", raw)
	}

	// instructions → system 消息，input → user 消息
	var msgs []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		t.Fatalf("unmarshal messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("messages len = %d, want 2 (system + user): %s", len(msgs), raw)
	}
	if msgs[0].Role != "system" || strings.TrimSpace(string(msgs[0].Content)) != `"You are helpful."` {
		t.Errorf("first message should be system with instructions, got role=%s content=%s", msgs[0].Role, msgs[0].Content)
	}
	if msgs[1].Role != "user" || !strings.Contains(string(msgs[1].Content), "你好") {
		t.Errorf("second message should be user with input text, got role=%s content=%s", msgs[1].Role, msgs[1].Content)
	}

	// stream 保留 + 注入 stream_options（usage 计费需要）
	if string(m["stream"]) != "true" {
		t.Errorf("stream should be preserved as true, got %s", m["stream"])
	}
	if _, ok := m["stream_options"]; !ok {
		t.Errorf("stream_options should be injected for stream request: %s", raw)
	}

	// ResponsesRequest 快照需落 RelayInfo，供响应侧合成 Responses 格式时 echo
	if info.ResponsesRequest == nil {
		t.Error("info.ResponsesRequest snapshot should be stashed by conversion")
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
