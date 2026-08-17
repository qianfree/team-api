package openai

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// chatViaResponsesMeta 实现 relaykit 能力接口（模型映射/请求 ID）的测试桩。
type chatViaResponsesMeta struct {
	convmeta.Values
	mapped    bool
	requestID string
}

func (m *chatViaResponsesMeta) ModelNameMapped() bool { return m.mapped }
func (m *chatViaResponsesMeta) GetRequestID() string  { return m.requestID }

// TestResponsesToChatResponse_Parity 对拍 legacy ResponsesResponseToChatCompletions
// （含 HandleResponsesNonStreamToChat 的模型名三段逻辑）与 relaykit 转换器，覆盖
// 模型映射 × 上游模型名存在 四象限。
// 已知差异（比较时排除）：id（legacy 用 info.RequestID，relaykit 同源——但桩两側一致故不排除）、
// created（双方 Now，非确定，排除）。
func TestResponsesToChatResponse_Parity(t *testing.T) {
	const body = `{
		"id": "resp_P1", "object": "response", "created_at": 1730000000, "status": "completed",
		"model": "UPSTREAM_MODEL",
		"output": [
			{"type": "message", "id": "msg_1", "status": "completed", "role": "assistant",
			 "content": [{"type": "output_text", "text": "回答"}]},
			{"type": "function_call", "id": "fc_1", "call_id": "call_9", "name": "f", "arguments": "{\"a\":1}", "status": "completed"}
		],
		"usage": {"input_tokens": 12, "output_tokens": 4, "total_tokens": 16,
			"input_tokens_details": {"cached_tokens": 2, "cache_write_tokens": 0, "audio_tokens": 0},
			"output_tokens_details": {"reasoning_tokens": 1, "audio_tokens": 0}}
	}`

	cases := []struct {
		name           string
		mapped         bool
		upstreamModel  string // 上游返回 model 为空时置 ""
		wantLegacySide string // 期望的最终 model 字段
	}{
		{"unmapped-with-upstream-model", false, "UPSTREAM_MODEL", "UPSTREAM_MODEL"},
		{"unmapped-without-upstream-model", false, "", "gpt-4o-2024-11-20"},
		{"mapped-with-upstream-model", true, "UPSTREAM_MODEL", "gpt-4o"},
		{"mapped-without-upstream-model", true, "", "gpt-4o"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// legacy：复刻 HandleResponsesNonStreamToChat 的完整模型名三段逻辑
			legacyInfo := &common.RelayInfo{
				RequestID:       "req-123",
				OriginModelName: "gpt-4o",
				ChannelMeta: &common.ChannelMeta{
					IsModelMapped:     tc.mapped,
					UpstreamModelName: "gpt-4o-2024-11-20",
				},
			}
			var resp dto.OpenAIResponsesResponse
			if err := json.Unmarshal([]byte(body), &resp); err != nil {
				t.Fatalf("parse: %v", err)
			}
			if tc.upstreamModel == "" {
				resp.Model = ""
			}
			model := legacyInfo.ChannelMeta.UpstreamModelName
			if !legacyInfo.ChannelMeta.IsModelMapped && resp.Model != "" {
				model = resp.Model
			}
			legacyChat, _, err := ResponsesResponseToChatCompletions(&resp, "chatcmpl-req-123", model)
			if err != nil {
				t.Fatalf("legacy convert: %v", err)
			}
			if legacyInfo.ChannelMeta.IsModelMapped {
				legacyChat.Model = legacyInfo.OriginModelName
			}

			// relaykit：经注册表公共 API（与生产桥接一致）
			kitMeta := &chatViaResponsesMeta{
				Values: convmeta.Values{
					ChannelMetaAttached: true,
					OriginModelName:     "gpt-4o",
					UpstreamModelName:   "gpt-4o-2024-11-20",
				},
				mapped:    tc.mapped,
				requestID: "req-123",
			}
			spec, ok := relayconvert.LookupTextConverter(relayconvert.ConverterOpenAIChatToOpenAIResponses)
			if !ok {
				t.Fatal("expected c2r converter registered")
			}
			kitAny, _, err := spec.Resp.Convert(context.Background(), kitMeta, &resp)
			if err != nil {
				t.Fatalf("relaykit convert: %v", err)
			}
			kitChat := kitAny.(*dto.ChatCompletionResponse)

			if kitChat.Model != tc.wantLegacySide {
				t.Errorf("relaykit model = %q, want %q", kitChat.Model, tc.wantLegacySide)
			}
			if kitChat.Model != legacyChat.Model {
				t.Errorf("model mismatch: legacy=%q relaykit=%q", legacyChat.Model, kitChat.Model)
			}
			// 深度比较（排除非确定时间戳）
			legacyChat.Created = 0
			kitChat.Created = 0
			if !reflect.DeepEqual(legacyChat, kitChat) {
				legacyJSON, _ := json.Marshal(legacyChat)
				kitJSON, _ := json.Marshal(kitChat)
				t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyJSON, kitJSON)
			}
		})
	}
}
