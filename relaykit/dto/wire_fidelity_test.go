// wire_fidelity_test.go — DTO wire 协议保真测试。
//
// 目的：验证 DTO 结构体与各上游 API 的真实 wire 格式一致。测试语料
// （testdata/wire/*.json）按官方 API 文档的字段形状手工编写，视为 wire 真相；
// 测试将语料解码进 DTO 再重新编码，逐 key 对比「语料 ⊆ 重编码结果」：
//   - key 丢失   → DTO 缺字段或 json tag 拼错（如历史上的 thoughtBudget/thinkingBudget）
//   - 值变形     → DTO 字段类型错误（如 wire 是对象却声明为 string）
//   - 解码报错   → DTO 类型与 wire 类型冲突
//
// 该测试是协议转换金样本套件的地基：转换测试的输入输出都以 DTO 表达，
// 若 DTO 本身偏离 wire 格式，金样本会把错误固化成「标准答案」。
// 新增/修改 DTO 字段时，请同步在语料中补充该字段的官方示例值。
package dto_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relaykit/dto"
)

// wireCases 语料清单：文件名 → 解码目标 DTO 类型。
// 新增语料文件后必须在此登记，未登记的 testdata/wire/*.json 会被测试报错提醒。
var wireCases = map[string]func(t *testing.T, data []byte) any{
	"openai_chat_request_full.json":  decodeInto[dto.GeneralOpenAIRequest],
	"openai_chat_response_full.json": decodeInto[dto.ChatCompletionResponse],
	"claude_request_full.json":       decodeInto[dto.ClaudeRequest],
	"claude_response_full.json":      decodeInto[dto.ClaudeResponse],
	"gemini_request_full.json":       decodeInto[dto.GeminiChatRequest],
	"gemini_response_full.json":      decodeInto[dto.GeminiChatResponse],
	"responses_request_full.json":    decodeInto[dto.OpenAIResponsesRequest],
	"responses_response_full.json":   decodeInto[dto.OpenAIResponsesResponse],
	"ollama_chat_request_full.json":  decodeInto[dto.OllamaChatRequest],
	"ollama_chat_response_full.json": decodeInto[dto.OllamaChatResponse],

	// 流式 chunk / 事件形状（流式转换器逐帧解码的目标类型）
	"openai_chat_stream_chunk_full.json":            decodeInto[dto.ChatCompletionStreamResponse],
	"claude_stream_event_message_start.json":        decodeInto[dto.ClaudeResponse],
	"claude_stream_event_content_block_start.json":  decodeInto[dto.ClaudeResponse],
	"claude_stream_event_content_block_delta.json":  decodeInto[dto.ClaudeResponse],
	"claude_stream_event_thinking_delta.json":       decodeInto[dto.ClaudeResponse],
	"claude_stream_event_signature_delta.json":      decodeInto[dto.ClaudeResponse],
	"claude_stream_event_message_delta.json":        decodeInto[dto.ClaudeResponse],
	"gemini_stream_chunk_full.json":                 decodeInto[dto.GeminiChatResponse],
	"responses_stream_event_text_delta.json":        decodeInto[dto.ResponsesStreamResponse],
	"responses_stream_event_output_item_added.json": decodeInto[dto.ResponsesStreamResponse],
	"responses_stream_event_reasoning_part.json":    decodeInto[dto.ResponsesStreamResponse],
}

func decodeInto[T any](t *testing.T, data []byte) any {
	t.Helper()
	var v T
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("解码进 DTO 失败（DTO 字段类型与 wire 冲突？）: %v", err)
	}
	return v
}

func TestWireFidelity(t *testing.T) {
	files, err := os.ReadDir(filepath.Join("testdata", "wire"))
	if err != nil {
		t.Fatalf("读取 testdata/wire 目录失败: %v", err)
	}

	seen := make(map[string]bool)
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		seen[f.Name()] = true
		decode, ok := wireCases[f.Name()]
		if !ok {
			t.Errorf("语料文件 %s 未在 wireCases 登记解码类型", f.Name())
			continue
		}
		t.Run(f.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "wire", f.Name()))
			if err != nil {
				t.Fatalf("读取语料失败: %v", err)
			}

			// wire 真相（语料原文）
			var want any
			if err := json.Unmarshal(data, &want); err != nil {
				t.Fatalf("语料不是合法 JSON: %v", err)
			}

			// wire → DTO → wire 往返
			v := decode(t, data)
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatalf("DTO 重编码失败: %v", err)
			}
			var got any
			if err := json.Unmarshal(out, &got); err != nil {
				t.Fatalf("重编码结果不是合法 JSON: %v", err)
			}

			// 语料 ⊆ 重编码结果：语料中的每个 key/值都必须在往返后幸存
			var diffs []string
			subsetDiff("$", want, got, &diffs)
			if len(diffs) > 0 {
				t.Errorf("DTO 往返丢失/变形了 %d 处 wire 字段：\n  %s",
					len(diffs), strings.Join(diffs, "\n  "))
			}
		})
	}

	for name := range wireCases {
		if !seen[name] {
			t.Errorf("wireCases 登记的语料文件 %s 不存在", name)
		}
	}
}

// subsetDiff 递归检查 want 的每个 key 路径在 got 中存在且值相等，差异追加进 diffs。
// want 中值为 null 的 key 在 got 中缺失视为等价（omitempty 指针字段的正常行为）。
func subsetDiff(path string, want, got any, diffs *[]string) {
	if want == nil {
		return // null ≈ 缺失/为 null，均可接受
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: wire 为对象，往返后为 %s", path, jsonTypeName(got)))
			return
		}
		for k, wv := range w {
			gv, exists := g[k]
			if !exists {
				if isJSONZero(wv) {
					// omitempty 对零值（0/""/false/空数组/空对象/null）的正常省略：
					// wire 语义上缺失 ≈ 零值，不算丢失。语料中语义敏感的字段请用非零值。
					continue
				}
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: key 丢失（DTO 缺字段或 tag 拼错）", path, k))
				continue
			}
			subsetDiff(path+"."+k, wv, gv, diffs)
		}
	case []any:
		g, ok := got.([]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: wire 为数组，往返后为 %s", path, jsonTypeName(got)))
			return
		}
		if len(g) != len(w) {
			*diffs = append(*diffs, fmt.Sprintf("%s: 数组长度 %d → %d", path, len(w), len(g)))
			return
		}
		for i := range w {
			subsetDiff(fmt.Sprintf("%s[%d]", path, i), w[i], g[i], diffs)
		}
	default:
		// 标量：两侧都经过 encoding/json，数字统一为 float64，直接比较
		if want != got {
			*diffs = append(*diffs, fmt.Sprintf("%s: 值变形 %v → %v", path, want, got))
		}
	}
}

// isJSONZero 判断 wire 值是否为 JSON 零值（omitempty 会省略的形态）。
func isJSONZero(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case float64:
		return t == 0
	case string:
		return t == ""
	case bool:
		return !t
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	}
	return false
}

func jsonTypeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		return "number"
	default:
		return fmt.Sprintf("%T", v)
	}
}
