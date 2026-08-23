package relaykit_bridge

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"

	// blank import 触发内置转换器注册（测试二进制须自备注册）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestTryConvertInboundToOpenAIChat_ResponsesInbound 回归（2026-08-21 收割误伤修复）：
// responses 入站 × ollama/coze/dify 等原生格式上游——handler 桥不路由该组合，
// 由共享 ConvertToOpenAI 经本桥完成 r2o 转换（legacy ConvertResponsesToOpenAI 收编）。
func TestTryConvertInboundToOpenAIChat_ResponsesInbound(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOllama),
			UpstreamModelName: "llama3",
		},
	}
	body := []byte(`{"model":"llama3","instructions":"be brief","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)

	out, ok, err := TryConvertInboundToOpenAIChat(context.Background(), info, body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected responses→openai conversion via shared bridge")
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("converted body not json: %v (%s)", err, out)
	}
	if _, ok := m["messages"]; !ok {
		t.Error("converted body missing messages")
	}
	if _, ok := m["input"]; ok {
		t.Error("responses input should be converted to chat messages")
	}
	// 快照 stash：供响应侧合成 Responses 格式时 echo 请求参数
	if info.ResponsesRequest == nil {
		t.Error("info.ResponsesRequest snapshot not stashed")
	}

	// 有状态请求（previous_response_id）：非 responses 原生上游无法还原——返回哨兵错误
	// 驱动 FSM 换渠道（经 ConvertToOpenAI → adaptor 透传至 relay_handler 哨兵判定点）
	stateful := []byte(`{"model":"llama3","previous_response_id":"resp_1","input":"hi"}`)
	if _, ok, err := TryConvertInboundToOpenAIChat(context.Background(), info, stateful); ok || err == nil {
		t.Error("stateful responses request should not convert via shared bridge")
	} else if !errors.Is(err, constant.ErrStatefulResponsesUnsupported) {
		t.Errorf("stateful error should wrap ErrStatefulResponsesUnsupported, got: %v", err)
	}
}

// TestTryConvertInboundToOpenAIChat_MalformedBodyError 已覆盖方向（claude/gemini/responses
// 入站）的畸形请求体必须显式报错（legacy 回退已收割，不得静默返回 ok=false）。
func TestTryConvertInboundToOpenAIChat_MalformedBodyError(t *testing.T) {
	for _, inbound := range []constant.RelayFormat{
		constant.RelayFormatClaude,
		constant.RelayFormatGemini,
		constant.RelayFormatResponses,
	} {
		info := &common.RelayInfo{
			InboundFormat: inbound,
			ChannelMeta: &common.ChannelMeta{
				ChannelType:       int(constant.ProviderOpenAI),
				UpstreamModelName: "gpt-4o",
			},
		}
		if _, ok, err := TryConvertInboundToOpenAIChat(context.Background(), info, []byte(`not-json`)); ok || err == nil {
			t.Errorf("%s inbound malformed body should return explicit error, got ok=%v err=%v", inbound, ok, err)
		}
	}
}

// TestTryConvertChatToResponsesRequestViaRelaykit ChatViaResponses 请求侧第二跳：
// 未置 UseResponsesAPI 时不得接管（防误伤普通 chat 渠道）。
func TestTryConvertChatToResponsesRequestViaRelaykit(t *testing.T) {
	info := &common.RelayInfo{
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			UpstreamModelName: "glm-4.7",
		},
	}
	chatBody := []byte(`{"model":"glm-4.7","max_tokens":512,"messages":[{"role":"user","content":"hi"}]}`)

	if _, ok := TryConvertChatToResponsesRequestViaRelaykit(context.Background(), info, chatBody); ok {
		t.Fatal("must not take over without UseResponsesAPI")
	}

	info.UseResponsesAPI = true
	out, ok := TryConvertChatToResponsesRequestViaRelaykit(context.Background(), info, chatBody)
	if !ok {
		t.Fatal("expected chat→responses conversion")
	}
	if strings.Contains(string(out), `"messages"`) {
		t.Errorf("chat messages should be converted to responses input: %s", out)
	}
	if !strings.Contains(string(out), `"input"`) {
		t.Errorf("responses body missing input: %s", out)
	}
}
