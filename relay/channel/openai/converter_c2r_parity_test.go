package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"reflect"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/relayconvert/convmeta"

	// blank import 触发内置转换器注册（与生产桥接路径一致）
	_ "github.com/qianfree/team-api/relaykit/relayconvert/register"
)

// TestConvertOpenAIToResponses_Parity 对拍 legacy c2r 与 relaykit c2r 的输出。
// legacy 完整路径 = adaptor 先对 chat 体注入 reasoning_effort（thinking 后缀场景）
// 再调 ConvertOpenAIToResponses；relaykit 转换器内部吸收了该注入语义。
func TestConvertOpenAIToResponses_Parity(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		modelMaped bool
		effort     string // 宿主 thinking 后缀映射（info.ReasoningEffort）
	}{
		{
			name: "basic",
			body: `{
				"model": "gpt-4o",
				"messages": [
					{"role": "system", "content": "你是一个助手"},
					{"role": "user", "content": "你好"}
				],
				"max_tokens": 512,
				"stream": true
			}`,
		},
		{
			name:       "model-mapped-and-effort-suffix",
			modelMaped: true,
			effort:     "high",
			body:       `{"model": "gpt-4o", "messages": [{"role": "user", "content": "hi"}]}`,
		},
		{
			name: "tools-history-format",
			body: `{
				"model": "gpt-4o",
				"messages": [
					{"role": "developer", "content": "仅中文"},
					{"role": "user", "content": "翻译"},
					{"role": "assistant", "content": null, "tool_calls": [{"id": "c1", "type": "function", "function": {"name": "lookup", "arguments": "{\"q\":1}"}}]},
					{"role": "tool", "tool_call_id": "c1", "content": "结果"},
					{"role": "user", "content": [{"type": "text", "text": "多模态"}, {"type": "image_url", "image_url": {"url": "https://e.com/a.png", "detail": "high"}}]}
				],
				"reasoning_effort": "low",
				"max_completion_tokens": 100,
				"prompt_cache_key": "s1",
				"user": "u1",
				"parallel_tool_calls": false,
				"response_format": {"type": "json_schema", "json_schema": {"name": "o", "schema": {"type": "object"}, "strict": true}},
				"tools": [{"type": "function", "function": {"name": "lookup", "description": "查询", "parameters": {"type": "object"}}}],
				"tool_choice": {"type": "function", "function": {"name": "lookup"}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstreamModel := "gpt-4o"
			if tt.modelMaped {
				upstreamModel = "gpt-4o-2024-11-20"
			}
			legacyInfo := &common.RelayInfo{
				ReasoningEffort: tt.effort,
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderOpenAI),
					IsModelMapped:     tt.modelMaped,
					UpstreamModelName: upstreamModel,
				},
			}
			// legacy 完整路径：先注入 reasoning_effort（adaptor.go:179-192 分支）再转换
			chatBody := []byte(tt.body)
			if legacyInfo.ReasoningEffort != "" {
				injected := injectReasoningEffort(bytes.NewReader(chatBody), legacyInfo.ReasoningEffort)
				injectedBody, readErr := io.ReadAll(injected)
				if readErr != nil {
					t.Fatalf("read injected body: %v", readErr)
				}
				chatBody = injectedBody
			}
			legacyOut, err := ConvertOpenAIToResponses(chatBody, legacyInfo)
			if err != nil {
				t.Fatalf("legacy convert: %v", err)
			}

			var chatReq dto.GeneralOpenAIRequest
			if err := json.Unmarshal([]byte(tt.body), &chatReq); err != nil {
				t.Fatalf("parse chat request: %v", err)
			}
			kitInfo := &convmeta.Values{
				ChannelMetaAttached: true,
				UpstreamModelName:   upstreamModel,
				ReasoningEffort:     tt.effort,
			}
			spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIChatToOpenAIResponses)
			if !ok {
				t.Fatal("expected c2r converter registered")
			}
			converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &chatReq)
			if err != nil {
				t.Fatalf("relaykit convert: %v", err)
			}
			kitOut, err := json.Marshal(converted)
			if err != nil {
				t.Fatalf("marshal relaykit body: %v", err)
			}

			var legacyMap, kitMap map[string]any
			if err := json.Unmarshal(legacyOut, &legacyMap); err != nil {
				t.Fatalf("unmarshal legacy: %v", err)
			}
			if err := json.Unmarshal(kitOut, &kitMap); err != nil {
				t.Fatalf("unmarshal relaykit: %v", err)
			}
			if !reflect.DeepEqual(legacyMap, kitMap) {
				t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyOut, kitOut)
			}
		})
	}
}
