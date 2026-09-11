package handler

// fuzz_test.go — 宿主入站解析 + 矩阵裁决 + 转换的模糊测试。
//
// 与 relaykit 侧的 fuzz（relayconvert/register/fuzz_test.go）覆盖面**不重叠**，
// 因为两者的输入形态不同：
//   - relaykit 侧直接用类型化 DTO 驱动转换器；
//   - 宿主 parseInboundRequest 是**裸 json.Unmarshal**，解析出的 message content
//     是 `[]any`（元素为 map[string]any）而非 `[]dto.ContentPart`。两种形态并存，
//     转换器须经 shared.NormalizeContentParts 归一——历史上多个静默能力丢失 bug
//     （多模态文本+图片丢失、链式 ContentPart 丢失）正出在这条缝上。
//
// 本目标按宿主真实路径构造输入：
//
//	parseInboundRequest（宿主 DTO）→ 矩阵裁决 converterID → ConvertRequestByID → marshal
//
// 断言：不 panic；转换成功 ⇒ 结果可 json.Marshal（宿主紧接着就要序列化发上游）。
//
// 不经 convertRequestViaRelaykit 外壳是有意为之：那层在转换失败时打 Warning 日志，
// 模糊测试每秒数十万次失败输入会产出海量日志、拖垮吞吐；被跳过的仅是日志与
// 指标上报，协议面逻辑（解析形态、矩阵、转换、序列化）全部覆盖。
//
// 运行方式：
//   - 常规回归：go test ./relay/handler/ —— 只跑种子语料；
//   - 主动模糊：go test -run XXX -fuzz FuzzHostInboundConversion ./relay/handler/

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/relaykit_bridge"
	"github.com/qianfree/team-api/relaykit/relayconvert"
)

// fuzzInbound 宿主支持的四种文本入站（格式 + 对应 RelayMode）。
var fuzzInbound = []struct {
	format constant.RelayFormat
	mode   constant.RelayMode
}{
	{constant.RelayFormatOpenAI, constant.RelayModeChatCompletions},
	{constant.RelayFormatClaude, constant.RelayModeClaudeMessages},
	{constant.RelayFormatGemini, constant.RelayModeGeminiChat},
	{constant.RelayFormatResponses, constant.RelayModeResponses},
}

// fuzzProviders 代表性上游供应商，覆盖全部上游协议格式
// （OpenAI / Claude / Gemini / Ollama），从而覆盖全部矩阵方向。
var fuzzProviders = []constant.ProviderType{
	constant.ProviderOpenAI,
	constant.ProviderClaude,
	constant.ProviderGemini,
	constant.ProviderOllama,
}

func fuzzRelayInfo(provider constant.ProviderType, inbound constant.RelayFormat, mode constant.RelayMode) *common.RelayInfo {
	return &common.RelayInfo{
		Context:         context.Background(),
		RequestID:       "fuzz",
		RelayMode:       int(mode),
		OriginModelName: "origin-model",
		InboundFormat:   inbound,
		ClientFormat:    inbound,
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(provider),
			BaseURL:           "https://upstream.invalid",
			ApiKey:            "sk-fuzz",
			UpstreamModelName: "upstream-model",
		},
	}
}

// FuzzParseInboundRequest 宿主入站解析：对任意字节都不得 panic。
// 这是客户端可直接控制的第一道门——解析失败是合法结果（宿主据此 hard-fail 拒绝请求），
// 崩溃则不是。
func FuzzParseInboundRequest(f *testing.F) {
	for _, body := range fuzzHostSeeds {
		for i := range fuzzInbound {
			f.Add(i, body)
		}
	}

	f.Fuzz(func(t *testing.T, inboundIdx int, body string) {
		if inboundIdx < 0 {
			return
		}
		in := fuzzInbound[inboundIdx%len(fuzzInbound)]

		parsed, err := parseInboundRequest(in.format, []byte(body))
		if err != nil {
			return // 解析失败是合法结果
		}
		if parsed == nil {
			t.Fatalf("parseInboundRequest(%s) 返回了 (nil, nil)：调用方会把 nil 传进转换器", in.format)
		}
	})
}

// FuzzHostInboundConversion 宿主全路径：解析 → 矩阵裁决 → 转换 → 序列化。
func FuzzHostInboundConversion(f *testing.F) {
	for _, body := range fuzzHostSeeds {
		for i := range fuzzInbound {
			f.Add(i, 0, body)
		}
	}

	f.Fuzz(func(t *testing.T, inboundIdx, providerIdx int, body string) {
		if inboundIdx < 0 || providerIdx < 0 {
			return
		}
		in := fuzzInbound[inboundIdx%len(fuzzInbound)]
		provider := fuzzProviders[providerIdx%len(fuzzProviders)]

		// 宿主真实解析路径：裸 unmarshal，content 为 []any(map) 形态
		parsed, err := parseInboundRequest(in.format, []byte(body))
		if err != nil || parsed == nil {
			return
		}

		info := fuzzRelayInfo(provider, in.format, in.mode)
		upstream := relaykit_bridge.EffectiveUpstreamFormat(info)
		converterID := relaykit_bridge.RequestConverterIDForRoute(in.format, upstream, info.RelayMode)
		if converterID == "" {
			return // 同格式直连或矩阵未覆盖，不经 relaykit
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		converted, err := relayconvert.ConvertRequestByID(ctx, info, converterID, parsed)
		if err != nil {
			return // 转换失败是合法结果（宿主 hard-fail 拒绝请求）
		}
		if converted == nil {
			t.Fatalf("转换器 %s 返回 (nil, nil)：宿主会 marshal nil 并向上游发出 \"null\"", converterID)
		}
		if _, err := json.Marshal(converted); err != nil {
			t.Fatalf("转换器 %s 产出不可序列化的结果（宿主 marshal 将 hard-fail 该请求）: %v", converterID, err)
		}
	})
}

// fuzzHostSeeds 种子语料：侧重宿主裸 unmarshal 才会产生的 []any content 形态，
// 以及各格式的多态/边界构造。
var fuzzHostSeeds = []string{
	// content 双形态：字符串 / 数组（宿主解析后为 []any(map)）/ 混杂元素
	`{"model":"m","messages":[{"role":"user","content":"hello"}]}`,
	`{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`,
	`{"model":"m","messages":[{"role":"user","content":[{"type":"text","text":"hi"},{"type":"image_url","image_url":{"url":"data:image/png;base64,AAA"}}]}]}`,
	`{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":""}}]}]}`,
	`{"model":"m","messages":[{"role":"user","content":[{"no_type":"x"}]}]}`,
	`{"model":"m","messages":[{"role":"user","content":[null,0,"",[],{}]}]}`,
	`{"model":"m","messages":[{"role":"user","content":[]}]}`,
	`{"model":"m","messages":[{"role":"user","content":null}]}`,
	// 工具循环
	`{"model":"m","messages":[{"role":"assistant","tool_calls":[{"id":"c1","type":"function","function":{"name":"f","arguments":"{}"}}]},{"role":"tool","tool_call_id":"c1","content":"ok"}]}`,
	`{"model":"m","messages":[],"tools":[{"type":"function","function":{"name":"f","parameters":{"type":"object"}}}],"tool_choice":{"type":"function","function":{"name":"f"}}}`,
	// Claude 原生
	`{"model":"m","max_tokens":1,"system":"s","messages":[{"role":"user","content":"hi"}]}`,
	`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}],"thinking":{"type":"enabled","budget_tokens":1024}}`,
	`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"r"}]}]}`,
	`{"model":"m","max_tokens":1,"messages":[],"tools":[{"type":"web_search_20250305","name":"web_search","max_uses":5}]}`,
	// Gemini 原生
	`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":1}}`,
	`{"contents":[{"role":"user","parts":[{"inlineData":{"mimeType":"image/png","data":"AAA"}}]}]}`,
	`{"contents":[{"role":"model","parts":[{"functionCall":{"name":"f","args":{}}}]}],"tools":[{"googleSearch":{}}]}`,
	`{"contents":[{"parts":[{"thought":true,"text":"t","thoughtSignature":"s"}]}]}`,
	// Responses 原生
	`{"model":"m","input":"plain"}`,
	`{"model":"m","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`,
	`{"model":"m","input":[{"type":"function_call","call_id":"c","name":"f","arguments":"{}"},{"type":"function_call_output","call_id":"c","output":"r"}]}`,
	`{"model":"m","input":[],"tools":[{"type":"web_search"}],"reasoning":{"effort":"medium"}}`,
	// 最小 / 畸形边界
	`{}`,
	`{"messages":null}`,
	`null`,
	`[]`,
	``,
}
