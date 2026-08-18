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

// c2oG2oParityInfo 构造对拍用 RelayInfo（convmeta.Meta + 无关字段）。
// 注意：ConvertToOpenAI 已被 relaykit 接管，对拍的 legacy 侧必须直调
// ConvertClaudeToOpenAI/ConvertGeminiToOpenAI 原函数，否则会变成 relaykit vs relaykit 的恒真式。
func c2oG2oParityInfo() *common.RelayInfo {
	return &common.RelayInfo{
		InboundFormat: constant.RelayFormatClaude,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			IsModelMapped:     false,
			UpstreamModelName: "gpt-4o-2024-11-20",
		},
	}
}

// TestC2O_Parity 对拍 legacy ConvertClaudeToOpenAI 与 relaykit spec A 转换器。
// 双方输出同构 struct（relay/dto 是 relaykit/dto 别名），unmarshal 成 map 后 DeepEqual。
func TestC2O_Parity(t *testing.T) {
	cases := []string{
		// 基础：system/多轮/max_tokens/temperature/stop
		`{"model":"m","max_tokens":2048,"temperature":0.5,"system":"你是助手","stop_sequences":["END"],
		"messages":[{"role":"user","content":"Hello"},{"role":"assistant","content":"你好"},{"role":"user","content":"Thanks"}]}`,
		// 工具 + thinking + system 数组 + tool_result 三形态
		`{"model":"m","max_tokens":8192,"thinking":{"type":"enabled","budget_tokens":20000},
		"system":[{"type":"text","text":"一"},{"type":"text","text":"二"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"x"}}],
		"messages":[
			{"role":"user","content":[{"type":"text","text":"查"},{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"aGk="}}]},
			{"role":"assistant","content":[{"type":"thinking","thinking":"想","signature":"s"},{"type":"text","text":"调用"},{"type":"tool_use","id":"t1","name":"f","input":{"a":1}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":[{"type":"text","text":"结果"}]},{"type":"text","text":"收到"}]}
		],
		"tools":[{"name":"f","description":"d","input_schema":{"type":"object"}}],
		"tool_choice":{"type":"tool","name":"f"}}`,
		// 怪癖：nil content、未知角色、tool_choice 字符串 "any"、input 缺失
		`{"model":"m","messages":[
			{"role":"user","content":null},
			{"role":"other","content":"丢弃"},
			{"role":"assistant","content":[{"type":"tool_use","id":"tx","name":"f"}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tx","content":{"map":"form"}}]}
		],"tool_choice":"any"}`,
	}

	legacyInfo := c2oG2oParityInfo()
	kitInfo := &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-4o-2024-11-20",
	}
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterClaudeMessagesToOpenAIChat)
	if !ok {
		t.Fatal("expected spec A registered")
	}

	for i, body := range cases {
		legacyReader, err := ConvertClaudeToOpenAI([]byte(body), legacyInfo)
		if err != nil {
			t.Fatalf("case %d legacy: %v", i, err)
		}
		legacyBody, err := io.ReadAll(legacyReader)
		if err != nil {
			t.Fatalf("case %d read: %v", i, err)
		}

		var claudeReq dto.ClaudeRequest
		if err := json.Unmarshal([]byte(body), &claudeReq); err != nil {
			t.Fatalf("case %d parse: %v", i, err)
		}
		converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &claudeReq)
		if err != nil {
			t.Fatalf("case %d relaykit: %v", i, err)
		}
		kitBody, err := json.Marshal(converted)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}

		var legacyMap, kitMap map[string]any
		_ = json.Unmarshal(legacyBody, &legacyMap)
		_ = json.Unmarshal(kitBody, &kitMap)
		if !reflect.DeepEqual(legacyMap, kitMap) {
			t.Errorf("case %d parity mismatch\nlegacy:  %s\nrelaykit: %s", i, legacyBody, kitBody)
		}
	}
}

// TestG2O_Parity 对拍 legacy ConvertGeminiToOpenAI 与 relaykit spec B 转换器。
func TestG2O_Parity(t *testing.T) {
	cases := []string{
		// 基础：systemInstruction/generationConfig 全参数
		`{"contents":[{"role":"user","parts":[{"text":"你好"}]}],
		"systemInstruction":{"parts":[{"text":"你是助手"},{"text":"仅中文"}]},
		"generationConfig":{"temperature":0.7,"topP":0.9,"topK":40,"maxOutputTokens":1024,"stopSequences":["END"],"seed":42}}`,
		// 工具历史：functionCall 同名两次 + functionResponse 反查 + pending 重排
		`{"contents":[
			{"role":"user","parts":[{"text":"跑"}]},
			{"role":"model","parts":[{"functionCall":{"name":"f","args":{"x":1}}},{"functionCall":{"name":"f","args":{"x":2}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"f","response":{"ok":true}}}]},
			{"role":"user","parts":[{"text":"完"}]}
		],
		"tools":[{"functionDeclarations":[{"name":"f","description":"d","parameters":{"type":"object"}}]}],
		"toolConfig":{"functionCallingConfig":{"mode":"ANY","allowedFunctionNames":["f"]}},
		"generationConfig":{"thinkingConfig":{"thoughtBudget":3000},"responseMimeType":"application/json"}}`,
		// 怪癖：未知 role 文本丢失、assistant 图文互斥、FileData 丢弃、args null
		`{"contents":[
			{"role":"user","parts":[{"text":"看"},{"inlineData":{"mimeType":"image/png","data":"aGk="}},{"fileData":{"fileUri":"gs://b/f","mimeType":"application/pdf"}}]},
			{"role":"strange","parts":[{"text":"丢"}]},
			{"role":"model","parts":[{"text":"一"},{"text":"二"},{"inlineData":{"mimeType":"image/png","data":"aW1n"}},{"functionCall":{"name":"noop","args":null}}]}
		]}`,
	}

	legacyInfo := c2oG2oParityInfo()
	legacyInfo.InboundFormat = constant.RelayFormatGemini
	kitInfo := &convmeta.Values{
		ChannelMetaAttached: true,
		UpstreamModelName:   "gpt-4o-2024-11-20",
	}
	spec, ok := relayconvert.LookupRequestConverter(relayconvert.ConverterGeminiContentToOpenAIChat)
	if !ok {
		t.Fatal("expected spec B registered")
	}

	for i, body := range cases {
		legacyReader, err := ConvertGeminiToOpenAI([]byte(body), legacyInfo)
		if err != nil {
			t.Fatalf("case %d legacy: %v", i, err)
		}
		legacyBody, err := io.ReadAll(legacyReader)
		if err != nil {
			t.Fatalf("case %d read: %v", i, err)
		}

		var geminiReq dto.GeminiChatRequest
		if err := json.Unmarshal([]byte(body), &geminiReq); err != nil {
			t.Fatalf("case %d parse: %v", i, err)
		}
		converted, err := relayconvert.ExecuteRequestConverter(context.Background(), spec, kitInfo, &geminiReq)
		if err != nil {
			t.Fatalf("case %d relaykit: %v", i, err)
		}
		kitBody, err := json.Marshal(converted)
		if err != nil {
			t.Fatalf("case %d marshal: %v", i, err)
		}

		var legacyMap, kitMap map[string]any
		_ = json.Unmarshal(legacyBody, &legacyMap)
		_ = json.Unmarshal(kitBody, &kitMap)
		if !reflect.DeepEqual(legacyMap, kitMap) {
			t.Errorf("case %d parity mismatch\nlegacy:  %s\nrelaykit: %s", i, legacyBody, kitBody)
		}
	}
}

// TestConvertToOpenAI_RelaykitTakesOver 验证 D1 接线：共享入口经 relaykit 接管
//（产物特征与 legacy 一致——同构输出，此测试主要保证接线路径可用且不回退）。
func TestConvertToOpenAI_RelaykitTakesOver(t *testing.T) {
	info := c2oG2oParityInfo()
	out, err := ConvertToOpenAI([]byte(`{"model":"m","max_tokens":100,"messages":[{"role":"user","content":"hi"}]}`), info)
	if err != nil {
		t.Fatalf("ConvertToOpenAI: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["model"] != "gpt-4o-2024-11-20" {
		t.Errorf("model = %v", m["model"])
	}
}
