package codex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// newTestInfo 构造测试用的 RelayInfo（Responses 模式 + chatgpt.com 官方后端）。
func newTestInfo(apiKeyJSON string) *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode: int(constant.RelayModeResponses),
		ChannelMeta: &common.ChannelMeta{
			BaseURL: "https://chatgpt.com",
			ApiKey:  apiKeyJSON,
		},
	}
}

const testCodexKey = `{"access_token":"atk123","account_id":"acc456","refresh_token":"rtk789"}`

func TestGetRequestURL_OfficialEndpoint(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)

	url, err := a.GetRequestURL(info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const wantSuffix = "/backend-api/codex/responses"
	if !strings.HasSuffix(url, wantSuffix) {
		t.Errorf("URL = %s, want suffix %s", url, wantSuffix)
	}
	if !strings.HasPrefix(url, "https://chatgpt.com") {
		t.Errorf("URL = %s, want base https://chatgpt.com", url)
	}
}

func TestGetRequestURL_RejectsNonResponses(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	info.RelayMode = int(constant.RelayModeChatCompletions)

	if _, err := a.GetRequestURL(info); err == nil {
		t.Fatal("expected error for non-Responses mode, got nil")
	}
}

func TestSetupRequestHeader_OfficialHeaders(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	a.Init(info)

	header := http.Header{}
	if err := a.SetupRequestHeader(header, info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// http.Header.Get 内部做 key 规范化，这里用原始大小写即可
	checks := map[string]string{
		"Authorization":      "Bearer atk123",
		"chatgpt-account-id": "acc456",
		"originator":         "codex_cli_rs",
		"OpenAI-Beta":        "responses=experimental",
		"Content-Type":       "application/json",
		"Accept":             "application/json",
	}
	for key, want := range checks {
		if got := header.Get(key); got != want {
			t.Errorf("header %s = %q, want %q", key, got, want)
		}
	}

	// 精确 Content-Type：不应带 charset 等参数
	if ct := header.Get("Content-Type"); strings.Contains(ct, ";") {
		t.Errorf("Content-Type should be exact media type, got %q", ct)
	}

	// 流式时 Accept 应切换为 text/event-stream
	info.IsStream = true
	streamHeader := http.Header{}
	if err := a.SetupRequestHeader(streamHeader, info); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := streamHeader.Get("Accept"); got != "text/event-stream" {
		t.Errorf("stream Accept = %q, want text/event-stream", got)
	}
}

func TestConvertRequest_FieldStripping(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	info.InboundFormat = constant.RelayFormatOpenAI

	body := `{"model":"gpt-5.1","input":"hello","store":true,` +
		`"max_output_tokens":1024,"temperature":0.7,` +
		`"reasoning":{"effort":"low"},"tools":[{"type":"function"}]}`

	reader, err := a.ConvertRequest(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read converted body failed: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (out=%s)", err, out)
	}

	// store 必须被强制为 false
	if string(got["store"]) != "false" {
		t.Errorf("store = %s, want false", got["store"])
	}
	// max_output_tokens / temperature 必须被移除
	if _, ok := got["max_output_tokens"]; ok {
		t.Error("max_output_tokens should be stripped")
	}
	if _, ok := got["temperature"]; ok {
		t.Error("temperature should be stripped")
	}
	// instructions 缺省补空串
	if string(got["instructions"]) != `""` {
		t.Errorf("instructions = %s, want \"\"", got["instructions"])
	}
	// 其余 Responses 字段应原样保留
	if string(got["input"]) != `"hello"` {
		t.Errorf("input field lost or altered: %s", got["input"])
	}
	if _, ok := got["reasoning"]; !ok {
		t.Error("reasoning field should be preserved")
	}
	if _, ok := got["tools"]; !ok {
		t.Error("tools field should be preserved")
	}
}

func TestConvertRequest_KeepsExistingInstructions(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	info.InboundFormat = constant.RelayFormatOpenAI

	body := `{"model":"gpt-5.1","input":"hi","instructions":"you are helpful"}`
	reader, err := a.ConvertRequest(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, _ := io.ReadAll(reader)

	var got map[string]json.RawMessage
	_ = json.Unmarshal(out, &got)
	if string(got["instructions"]) != `"you are helpful"` {
		t.Errorf("existing instructions overwritten: %s", got["instructions"])
	}
}

func TestConvertRequest_ModelMapping(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	info.InboundFormat = constant.RelayFormatOpenAI
	info.ChannelMeta.IsModelMapped = true
	info.ChannelMeta.UpstreamModelName = "gpt-5.6-codex"

	body := `{"model":"user-alias","input":"hi"}`
	reader, err := a.ConvertRequest(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, _ := io.ReadAll(reader)

	var got map[string]json.RawMessage
	_ = json.Unmarshal(out, &got)
	if string(got["model"]) != `"gpt-5.6-codex"` {
		t.Errorf("model = %s, want \"gpt-5.6-codex\"", got["model"])
	}
}

// TestConvertRequest_ResponsesInboundKeepsResponsesBody 回归：responses 入站 +
// 模型映射（canPassThrough 被关闭）时，Responses 体必须原样进入字段手术——
// 此前会被 ConvertToOpenAI 错转成 chat 体再发往 Responses 专用端点（上游拒绝）。
func TestConvertRequest_ResponsesInboundKeepsResponsesBody(t *testing.T) {
	a := &Adaptor{}
	info := newTestInfo(testCodexKey)
	info.InboundFormat = constant.RelayFormatResponses
	info.ChannelMeta.IsModelMapped = true
	info.ChannelMeta.UpstreamModelName = "gpt-5.6-codex"

	body := `{"model":"user-alias","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"temperature":0.7}`
	reader, err := a.ConvertRequest(context.Background(), info, []byte(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, _ := io.ReadAll(reader)

	var got map[string]json.RawMessage
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("converted body is not json: %v, body: %s", err, out)
	}
	if _, ok := got["input"]; !ok {
		t.Errorf("responses input should be preserved: %s", out)
	}
	if _, ok := got["messages"]; ok {
		t.Errorf("must NOT convert to chat messages（responses 体被错转）: %s", out)
	}
	if string(got["store"]) != "false" {
		t.Errorf("store = %s, want false（字段手术应执行）", got["store"])
	}
	if _, ok := got["temperature"]; ok {
		t.Errorf("temperature should be stripped: %s", out)
	}
	if string(got["model"]) != `"gpt-5.6-codex"` {
		t.Errorf("model = %s, want gpt-5.6-codex", got["model"])
	}
}
