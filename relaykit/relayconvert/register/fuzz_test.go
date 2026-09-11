package register

// fuzz_test.go — 协议转换的模糊测试（注册表级，覆盖全部已注册方向）。
//
// 为什么需要：宿主对客户端请求体是**裸 json.Unmarshal**，解析出的 content 有
// `[]any`(map) 与 `[]dto.ContentPart` 两种并存形态，转换器要靠
// shared.NormalizeContentParts 归一；再加上各格式大量可选/多态字段
//（Responses 的 action 多态、Claude 的 content 块联合体、Gemini 的 parts 联合体），
// 「结构合法但语义刁钻」的输入空间远大于手写语料能覆盖的范围。
//
// 断言两条不变量（对任意输入都必须成立）：
//  1. 不 panic——转换器崩溃会让该请求 500，且在共享进程里污染日志与指标；
//  2. 转换成功 ⇒ 结果必须可 json.Marshal——宿主紧接着就要序列化发往上游
//     （relaykit_bridge 的 marshal 失败是 hard-fail），产出不可序列化的值
//     （NaN/Inf 浮点、含 chan/func 的字段）等同于静默制造 500。
//
// 运行方式：
//   - 常规回归：go test ./relayconvert/register/ —— 只跑种子语料（含金样本全部输入）；
//   - 主动模糊：go test -run XXX -fuzz FuzzRequestConversion ./relayconvert/register/
//     失败输入由 go 自动落盘到 testdata/fuzz/<FuzzName>/，随仓库提交后即成为永久回归种子。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relaykit/dto"
	"github.com/qianfree/team-api/relaykit/relayconvert"
	"github.com/qianfree/team-api/relaykit/types"
)

// fuzzFormats 模糊测试覆盖的入站格式（与宿主 parseInboundRequest 支持的集合一致）。
var fuzzFormats = []types.RelayFormat{
	types.RelayFormatOpenAI,
	types.RelayFormatClaude,
	types.RelayFormatGemini,
	types.RelayFormatOpenAIResponses,
}

// fuzzParseRequest 按格式解析请求体（解析失败返回 nil，由调用方跳过——
// 宿主对解析失败是 hard-fail 拒绝请求，不进入转换器）。
func fuzzParseRequest(format types.RelayFormat, body []byte) any {
	switch format {
	case types.RelayFormatOpenAI:
		var req dto.GeneralOpenAIRequest
		if json.Unmarshal(body, &req) != nil {
			return nil
		}
		return &req
	case types.RelayFormatClaude:
		var req dto.ClaudeRequest
		if json.Unmarshal(body, &req) != nil {
			return nil
		}
		return &req
	case types.RelayFormatGemini:
		var req dto.GeminiChatRequest
		if json.Unmarshal(body, &req) != nil {
			return nil
		}
		return &req
	case types.RelayFormatOpenAIResponses:
		var req dto.OpenAIResponsesRequest
		if json.Unmarshal(body, &req) != nil {
			return nil
		}
		return &req
	}
	return nil
}

// addSeedsFromDir 把目录下匹配后缀的语料按「文件名前缀 → 格式下标」加入种子集。
// 文件名形如 <格式>__<场景>.<ext>，与金样本套件共用同一份语料。
func addSeedsFromDir(f *testing.F, dir, ext string) {
	f.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return // 语料目录缺失不阻断模糊测试（仍有手写种子）
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ext) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		formatName := strings.SplitN(strings.TrimSuffix(e.Name(), ext), "__", 2)[0]
		for i, format := range fuzzFormats {
			if string(format) == formatName {
				f.Add(i, string(data))
				break
			}
		}
	}
}

// fuzzAdversarialSeeds 手写对抗种子：结构合法但语义刁钻，专打已知易错点。
var fuzzAdversarialSeeds = []struct {
	formatIdx int
	body      string
}{
	// content 双形态：字符串 / 数组 / 空数组 / 类型错位的元素
	{0, `{"model":"m","messages":[{"role":"user","content":[]}]}`},
	{0, `{"model":"m","messages":[{"role":"user","content":[{"type":"text"}]}]}`},
	{0, `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{}}]}]}`},
	{0, `{"model":"m","messages":[{"role":"user","content":[null,123,"x"]}]}`},
	{0, `{"model":"m","messages":[{"role":"user","content":{}}]}`},
	// 工具调用：空参数 / 非法 JSON 参数字符串 / 缺 id
	{0, `{"model":"m","messages":[{"role":"assistant","tool_calls":[{"id":"","type":"function","function":{"name":"","arguments":""}}]}]}`},
	{0, `{"model":"m","messages":[{"role":"assistant","tool_calls":[{"id":"c","type":"function","function":{"name":"f","arguments":"{not json"}}]}]}`},
	{0, `{"model":"m","messages":[{"role":"tool","tool_call_id":"nonexistent","content":"r"}]}`},
	// tool_choice 多态
	{0, `{"model":"m","messages":[],"tool_choice":{"type":"function","function":{"name":"f"}}}`},
	{0, `{"model":"m","messages":[],"tool_choice":"required"}`},
	// 极端数值（NaN/Inf 无法用 JSON 表达，但超大数与负值可以）
	{0, `{"model":"m","messages":[],"temperature":1e308,"max_tokens":-1,"top_p":-5}`},
	// Claude：空 content 块数组 / thinking 块 / 服务端工具
	{1, `{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[]}]}`},
	{1, `{"model":"m","max_tokens":1,"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"t","signature":""}]}]}`},
	{1, `{"model":"m","max_tokens":1,"messages":[],"tools":[{"type":"web_search_20250305","name":"web_search"}]}`},
	{1, `{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"x","content":[]}]}]}`},
	{1, `{"model":"m","max_tokens":0,"messages":[],"system":[]}`},
	// Gemini：空 parts / 多态 part / 无 role
	{2, `{"contents":[{"parts":[]}]}`},
	{2, `{"contents":[{"role":"user","parts":[{"inlineData":{"mimeType":"","data":""}}]}]}`},
	{2, `{"contents":[{"role":"model","parts":[{"functionCall":{"name":"","args":null}}]}]}`},
	{2, `{"contents":[],"tools":[{"googleSearch":{}}],"generationConfig":{"thinkingConfig":{"thinkingBudget":-1}}}`},
	// Responses：input 字符串形态 / 多态 action / 空 output
	{3, `{"model":"m","input":"plain string input"}`},
	{3, `{"model":"m","input":[{"type":"function_call","call_id":"c","name":"f","arguments":""}]}`},
	{3, `{"model":"m","input":[{"role":"user","content":[]}],"tools":[{"type":"web_search"}]}`},
	{3, `{"model":"m","input":[],"reasoning":{"effort":"high"},"max_output_tokens":0}`},
	// 空对象与空数组（最小合法输入）
	{0, `{}`}, {1, `{}`}, {2, `{}`}, {3, `{}`},
}

// FuzzRequestConversion 对全部已注册请求转换方向做模糊测试。
func FuzzRequestConversion(f *testing.F) {
	addSeedsFromDir(f, filepath.Join("golden", "inputs"), ".json")
	for _, s := range fuzzAdversarialSeeds {
		f.Add(s.formatIdx, s.body)
	}

	f.Fuzz(func(t *testing.T, formatIdx int, body string) {
		if formatIdx < 0 {
			return
		}
		format := fuzzFormats[formatIdx%len(fuzzFormats)]

		parsed := fuzzParseRequest(format, []byte(body))
		if parsed == nil {
			return // 解析失败：宿主在进入转换器前就已拒绝
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, id := range relayconvert.ListRequestConverterIDs() {
			spec, ok := relayconvert.LookupRequestConverter(id)
			if !ok || spec.From != format {
				continue
			}

			// 不变量 1：不得 panic（panic 会直接冒泡为测试失败并记录输入）
			converted, err := relayconvert.ConvertRequestByID(ctx, capMeta(), id, parsed)
			if err != nil {
				continue // 返回错误是合法行为（宿主按 hard-fail 拒绝请求）
			}
			if converted == nil {
				t.Fatalf("转换器 %s 返回了 (nil, nil)：宿主会对 nil 做 marshal 并发出 \"null\" 请求体", id)
			}

			// 不变量 2：成功的结果必须可序列化（宿主紧接着 json.Marshal 发往上游）
			if _, err := json.Marshal(converted); err != nil {
				t.Fatalf("转换器 %s 产出了不可序列化的结果（宿主 marshal 将 hard-fail 该请求）: %v", id, err)
			}
		}
	})
}

// FuzzResponseConversion 对全部已注册响应转换方向做模糊测试
// （输入为上游响应体，由上游供应商控制——网关不能假设其良构）。
func FuzzResponseConversion(f *testing.F) {
	// 上游格式集合比入站多一个 Ollama
	upstreamFormats := append(append([]types.RelayFormat{}, fuzzFormats...), types.RelayFormatOllama)

	seeds := []struct {
		idx  int
		body string
	}{
		{0, `{"id":"c","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`},
		{0, `{"choices":[]}`},
		{0, `{"choices":[{"index":0,"message":{"role":"assistant","content":null,"tool_calls":[{"id":"","type":"function","function":{"name":"","arguments":"{bad"}}]}}]}`},
		{1, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`},
		{1, `{"type":"message","content":[]}`},
		{1, `{"type":"message","content":[{"type":"tool_use","id":"","name":"","input":null}]}`},
		{2, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2}}`},
		{2, `{"candidates":[]}`},
		{2, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"f"}}]},"finishReason":"SAFETY"}]}`},
		{2, `{"promptFeedback":{"blockReason":"SAFETY"}}`},
		{3, `{"id":"r","object":"response","status":"completed","output":[{"type":"message","id":"m","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`},
		{3, `{"status":"failed","error":{"code":"e","message":"m"},"output":[]}`},
		{4, `{"model":"m","message":{"role":"assistant","content":"hi"},"done":true,"prompt_eval_count":1,"eval_count":1}`},
		{4, `{"done":false}`},
		{0, `{}`}, {1, `{}`}, {2, `{}`}, {3, `{}`}, {4, `{}`},
	}
	for _, s := range seeds {
		f.Add(s.idx, s.body)
	}

	f.Fuzz(func(t *testing.T, formatIdx int, body string) {
		if formatIdx < 0 {
			return
		}
		upstream := upstreamFormats[formatIdx%len(upstreamFormats)]

		parsed := fuzzParseResponse(upstream, []byte(body))
		if parsed == nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, id := range relayconvert.ListResponseConverterIDs() {
			spec, ok := relayconvert.LookupResponseConverter(id)
			if !ok || spec.Convert == nil || spec.To != upstream {
				continue
			}

			converted, _, err := spec.Convert(ctx, capMeta(), parsed)
			if err != nil {
				continue
			}
			if converted == nil {
				t.Fatalf("响应转换器 %s 返回了 (nil, nil)", id)
			}
			if _, err := json.Marshal(converted); err != nil {
				t.Fatalf("响应转换器 %s 产出了不可序列化的结果: %v", id, err)
			}
		}
	})
}

// fuzzParseResponse 按上游格式解析响应体（解析失败返回 nil）。
func fuzzParseResponse(format types.RelayFormat, body []byte) any {
	switch format {
	case types.RelayFormatOpenAI:
		var r dto.ChatCompletionResponse
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &r
	case types.RelayFormatClaude:
		var r dto.ClaudeResponse
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &r
	case types.RelayFormatGemini:
		var r dto.GeminiChatResponse
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &r
	case types.RelayFormatOpenAIResponses:
		var r dto.OpenAIResponsesResponse
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &r
	case types.RelayFormatOllama:
		var r dto.OllamaChatResponse
		if json.Unmarshal(body, &r) != nil {
			return nil
		}
		return &r
	}
	return nil
}

// FuzzStreamConversion 对全部已注册流式方向做模糊测试。
// 输入为上游 SSE / NDJSON 原始字节——由上游供应商控制，且流式解析器自己做分帧，
// 是「畸形输入」暴露面最大的一段（半个事件、超长行、嵌套引号、截断 JSON）。
func FuzzStreamConversion(f *testing.F) {
	streamFormats := []types.RelayFormat{
		types.RelayFormatOpenAI,
		types.RelayFormatClaude,
		types.RelayFormatGemini,
		types.RelayFormatOpenAIResponses,
		types.RelayFormatOllama,
	}

	// 种子：金样本流式语料（文件名前缀即上游格式）
	entries, _ := os.ReadDir(filepath.Join("golden", "stream_inputs"))
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".txt") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("golden", "stream_inputs", e.Name()))
		if err != nil {
			continue
		}
		formatName := strings.SplitN(strings.TrimSuffix(e.Name(), ".txt"), "__", 2)[0]
		for i, format := range streamFormats {
			if string(format) == formatName {
				f.Add(i, string(data))
				break
			}
		}
	}

	// 对抗种子：畸形分帧
	for _, s := range []struct {
		idx  int
		body string
	}{
		{0, "data: \n\n"},
		{0, "data: {\n\n"},
		{0, "data: [DONE]\n\n"},
		{0, "data: null\n\n"},
		{0, "garbage without prefix\n\n"},
		{0, "data: {\"choices\":[]}\n\ndata: [DONE]\n\n"},
		{1, "event: message_start\ndata: {}\n\n"},
		{1, "event: error\ndata: {\"type\":\"error\",\"error\":{}}\n\n"},
		{1, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":99,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\"}}\n\n"},
		{1, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"},
		{2, "data: {\"candidates\":null}\n\n"},
		{2, "data: {\"candidates\":[{\"content\":{\"parts\":null}}]}\n\n"},
		{3, "event: response.completed\ndata: {\"type\":\"response.completed\"}\n\n"},
		{3, "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{}}\n\n"},
		{4, "{\"done\":true}\n"},
		{4, "not json\n{\"done\":true}\n"},
		{0, ""},
	} {
		f.Add(s.idx, s.body)
	}

	f.Fuzz(func(t *testing.T, formatIdx int, data string) {
		if formatIdx < 0 {
			return
		}
		upstream := streamFormats[formatIdx%len(streamFormats)]

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, spec := range relayconvert.ListStreamConverterSpecs() {
			if spec.From != upstream {
				continue
			}
			fn, ok := relayconvert.LookupStreamConverterByID(spec.ID)
			if !ok {
				continue
			}

			// 不变量：不 panic；每个写出的 chunk 必须可序列化
			//（宿主经 WriteSSEDataJSON/WriteSSEEventJSON 序列化，失败即该帧被丢弃，
			// 客户端收到不完整的流却看不到任何错误）
			_ = fn(ctx, capMeta(), strings.NewReader(data), func(chunk any) error {
				if chunk == nil {
					return nil
				}
				payload := chunk
				if ev, isEvent := chunk.(*relayconvert.StreamEvent); isEvent {
					if ev == nil || ev.Data == nil {
						return nil
					}
					payload = ev.Data
				}
				if _, err := json.Marshal(payload); err != nil {
					t.Fatalf("流式转换器 %s 产出了不可序列化的 chunk（宿主会静默丢帧）: %v", spec.ID, err)
				}
				return nil
			})
		}
	})
}
