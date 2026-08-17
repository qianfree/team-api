package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestChatToResponsesResponse_Parity 对拍 legacy chatCompletionToResponsesResponse 与
// relaykit OpenAIChatToResponsesResponseConverter。legacy 侧走 adaptor 非流式 handler
// 全路径（该路径已接入 relaykit 优先 + 回退，产物即 relaykit 输出），因此另以
// 「强制 legacy」方式对拍：直接调 chatCompletionToResponsesResponse 函数。
// 已知差异（比较时排除）：completed_at（legacy=created+1，relaykit=created——确定性修复项）。
func TestChatToResponsesResponse_Parity(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "text-with-usage-details",
			body: `{
				"id": "chatcmpl-A1", "object": "chat.completion", "created": 1730000000, "model": "gpt-4o-2024",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "你好"}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 100, "completion_tokens": 20, "total_tokens": 120,
					"prompt_tokens_details": {"cached_tokens": 40, "cache_write_tokens": 5, "audio_tokens": 0},
					"completion_tokens_details": {"reasoning_tokens": 3, "audio_tokens": 1, "rejected_prediction_tokens": 2}}
			}`,
		},
		{
			name: "tools-and-empty-content",
			body: `{
				"id": "chatcmpl-B2", "object": "chat.completion", "created": 1730000100, "model": "glm-4.6",
				"choices": [{"index": 0, "message": {"role": "assistant", "content": "",
					"tool_calls": [{"id": "call_1", "type": "function", "function": {"name": "f", "arguments": "{\"a\":1}"}}]},
					"finish_reason": "tool_calls"}],
				"usage": {"prompt_tokens": 50, "completion_tokens": 10, "total_tokens": 60}
			}`,
		},
		{
			name: "non-string-content",
			body: `{
				"id": "chatcmpl-C3", "object": "chat.completion", "created": 1730000200, "model": "m",
				"choices": [{"index": 0, "message": {"role": "assistant",
					"content": [{"type": "text", "text": "多模态文本"}]}, "finish_reason": "stop"}],
				"usage": {"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// legacy：直接调转换函数（adaptor handler 已接入 relaykit 优先，全路径产物即 relaykit 输出）
			legacyInfo := &common.RelayInfo{
				OriginModelName: "gpt-4o",
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderOpenAI),
					IsModelMapped:     false,
					UpstreamModelName: "gpt-4o",
				},
			}
			var chatResp dto.ChatCompletionResponse
			if err := json.Unmarshal([]byte(tt.body), &chatResp); err != nil {
				t.Fatalf("parse chat response: %v", err)
			}
			legacyBody, err := json.Marshal(chatCompletionToResponsesResponse(&chatResp, legacyInfo))
			if err != nil {
				t.Fatalf("marshal legacy: %v", err)
			}

			// relaykit：经注册表公共 API（与生产桥接一致的执行路径）
			kitInfo := &convmeta.Values{
				ChannelMetaAttached: true,
				OriginModelName:     "gpt-4o",
				UpstreamModelName:   "gpt-4o",
			}
			spec, ok := relayconvert.LookupTextConverter(relayconvert.ConverterOpenAIResponsesToOpenAIChat)
			if !ok {
				t.Fatal("expected r2c converter registered")
			}
			kitRespAny, _, err := spec.Resp.Convert(context.Background(), kitInfo, &chatResp)
			if err != nil {
				t.Fatalf("relaykit convert: %v", err)
			}
			kitBody, err := json.Marshal(kitRespAny)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			var legacyMap, kitMap map[string]any
			if err := json.Unmarshal(legacyBody, &legacyMap); err != nil {
				t.Fatalf("unmarshal legacy: %v", err)
			}
			if err := json.Unmarshal(kitBody, &kitMap); err != nil {
				t.Fatalf("unmarshal relaykit: %v", err)
			}
			// 排除确定性修复项：completed_at（legacy=created+1，relaykit=created）
			delete(legacyMap, "completed_at")
			delete(kitMap, "completed_at")
			if !reflect.DeepEqual(legacyMap, kitMap) {
				t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyBody, kitBody)
			}
		})
	}
}

// TestResponsesInboundNonStream_BridgeTakesOver 验证接线：chat 上游 + responses 客户端
// 场景下 relaykit 桥接接管全路径（而非回退 legacy 内联转换）。
func TestResponsesInboundNonStream_BridgeTakesOver(t *testing.T) {
	info := &common.RelayInfo{
		OriginModelName: "gpt-4o",
		InboundFormat:   constant.RelayFormatResponses,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			UpstreamModelName: "gpt-4o",
		},
	}
	resp := &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{
		"id": "chatcmpl-T1", "object": "chat.completion", "created": 1730000000, "model": "m",
		"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
		"usage": {"prompt_tokens": 8, "completion_tokens": 2, "total_tokens": 10}
	}`))}
	recorder := httptest.NewRecorder()
	adaptor := &Adaptor{}
	usage, err := adaptor.handleResponsesInboundNonStream(context.Background(), resp, info, recorder)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if usage.PromptTokens != 8 || usage.CompletionTokens != 2 {
		t.Errorf("usage = %+v, want prompt=8 completion=2（桥接口径带 details）", usage)
	}
	// 桥接产物特征：completed_at == created_at（legacy 为 +1）
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["completed_at"] != body["created_at"] {
		t.Errorf("completed_at = %v, created_at = %v（桥接接管时二者应相等）", body["completed_at"], body["created_at"])
	}
}
