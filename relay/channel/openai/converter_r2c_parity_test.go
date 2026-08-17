package openai

import (
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

// TestConvertResponsesToOpenAI_Parity 对拍 legacy 转换器与 relaykit 转换器的输出：
// 同输入双跑，双方产物均 unmarshal 为 map 后 DeepEqual（消除 key 顺序差异）。
// 已知可忽略的序列化形态差（工具缺 description/parameters 时 legacy 输出 null、
// relaykit 省略键）不在对拍用例范围内——用例均使用完整字段。
func TestConvertResponsesToOpenAI_Parity(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		modelMaped bool // 渠道是否做了模型映射
	}{
		{
			name: "basic",
			body: `{
				"model": "gpt-4o",
				"instructions": "你是一个翻译助手",
				"input": "Hello world",
				"max_output_tokens": 1024,
				"temperature": 0.7,
				"top_p": 0.9,
				"stream": true
			}`,
		},
		{
			name:       "model-mapped",
			modelMaped: true,
			body:       `{"model": "gpt-4o", "input": "hi"}`,
		},
		{
			name: "tools-and-format",
			body: `{
				"model": "gpt-4o",
				"input": [{"type": "message", "role": "user", "content": "天气怎么样"}],
				"tools": [{
					"type": "function",
					"name": "get_weather",
					"description": "查询天气",
					"parameters": {"type": "object", "properties": {"city": {"type": "string"}}}
				}],
				"tool_choice": {"type": "function", "function": {"name": "get_weather"}},
				"reasoning": {"effort": "medium"},
				"text": {"format": {"type": "json_object"}},
				"frequency_penalty": 0.1,
				"presence_penalty": 0.2
			}`,
		},
		{
			name: "stateless-history",
			body: `{
				"model": "gpt-4o",
				"input": [
					{"type": "message", "role": "developer", "content": "仅中文"},
					{"type": "message", "role": "user", "content": "写加法函数"},
					{"type": "reasoning", "id": "rs_1", "summary": [], "encrypted_content": "e30="},
					{"type": "function_call", "call_id": "call_1", "name": "run", "arguments": "{\"a\":1}"},
					{"type": "function_call", "call_id": "call_2", "name": "lint", "arguments": "{\"b\":2}"},
					{"type": "function_call_output", "call_id": "call_1", "output": "PASS"},
					{"type": "function_call_output", "call_id": "call_2", "output": "OK"}
				],
				"text": {"format": {"type": "json_schema", "name": "r", "schema": {"type": "object"}, "strict": true}}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// catalog 语义：未映射渠道 UpstreamModelName = 客户端模型名（internal/dispatchadapter/catalog.go）
			upstreamModel := "gpt-4o"
			if tt.modelMaped {
				upstreamModel = "gpt-4o-2024-11-20"
			}
			legacyInfo := &common.RelayInfo{
				IsStream: true,
				ChannelMeta: &common.ChannelMeta{
					ChannelType:       int(constant.ProviderOpenAI),
					IsModelMapped:     tt.modelMaped,
					UpstreamModelName: upstreamModel,
				},
			}
			// legacy 完整路径 = 转换器 + adaptor 后处理链（relay_handler relaykit 接管后被跳过的部分）：
			// ConvertResponsesToOpenAI → replaceModelIfNeeded → InjectStreamOptions（ReasoningEffort 为空跳过）
			legacyReader, err := ConvertResponsesToOpenAI([]byte(tt.body), legacyInfo)
			if err != nil {
				t.Fatalf("legacy convert: %v", err)
			}
			legacyReader = replaceModelIfNeeded(legacyReader, legacyInfo)
			legacyReader = InjectStreamOptions(legacyReader, legacyInfo)
			legacyBody, err := io.ReadAll(legacyReader)
			if err != nil {
				t.Fatalf("read legacy body: %v", err)
			}

			var responsesReq dto.OpenAIResponsesRequest
			if err := json.Unmarshal([]byte(tt.body), &responsesReq); err != nil {
				t.Fatalf("parse responses request: %v", err)
			}
			kitInfo := &convmeta.Values{
				ChannelMetaAttached: true,
				UpstreamModelName:   upstreamModel,
				IsStream:            true,
			}
			// 经注册表 + 链执行器调用（与生产桥接路径一致；internal 包不允许宿主直接导入）
			spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterOpenAIResponsesToOpenAIChat)
			if !ok {
				t.Fatal("expected r2c converter registered")
			}
			converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &responsesReq)
			if err != nil {
				t.Fatalf("relaykit convert: %v", err)
			}
			kitBody, err := json.Marshal(converted)
			if err != nil {
				t.Fatalf("marshal relaykit body: %v", err)
			}

			var legacyMap, kitMap map[string]any
			if err := json.Unmarshal(legacyBody, &legacyMap); err != nil {
				t.Fatalf("unmarshal legacy body: %v", err)
			}
			if err := json.Unmarshal(kitBody, &kitMap); err != nil {
				t.Fatalf("unmarshal relaykit body: %v", err)
			}
			if !reflect.DeepEqual(legacyMap, kitMap) {
				t.Errorf("parity mismatch\nlegacy:  %s\nrelaykit: %s", legacyBody, kitBody)
			}
		})
	}
}
