package claude

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// geminiInboundInfo 构造 Gemini 入站 + Claude 上游渠道的 RelayInfo
func geminiInboundInfo(isStream bool) *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(constant.RelayModeGeminiChat),
		IsStream:        isStream,
		RequestID:       "req123",
		InboundFormat:   constant.RelayFormatGemini,
		ClientFormat:    constant.RelayFormatGemini,
		OriginModelName: "claude-sonnet-5",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderClaude),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "claude-sonnet-5",
		},
	}
}

func geminiJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       geminiReadCloser(body),
	}
}

func geminiReadCloser(s string) *geminiNopCloser {
	return &geminiNopCloser{Reader: strings.NewReader(s)}
}

type geminiNopCloser struct{ *strings.Reader }

func (n *geminiNopCloser) Close() error { return nil }

// parseGeminiSSE 解析 Gemini 格式 SSE 输出（只有 data 行，无 event 行）为 chunk 列表。
// 返回解析后的 chunk 与是否收到 [DONE]。
func parseGeminiSSE(t *testing.T, raw string) ([]dto.GeminiChatResponse, bool) {
	t.Helper()
	var (
		chunks   []dto.GeminiChatResponse
		gotDone  bool
		frameSep = "\n\n"
	)
	for frame := range strings.SplitSeq(raw, frameSep) {
		frame = strings.TrimSpace(frame)
		if frame == "" {
			continue
		}
		payload, ok := strings.CutPrefix(frame, "data: ")
		if !ok {
			t.Fatalf("非法 SSE 帧（缺 data 前缀）: %q", frame)
		}
		if payload == "[DONE]" {
			gotDone = true
			continue
		}
		var chunk dto.GeminiChatResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			t.Fatalf("解析 Gemini chunk 失败: %v\ndata=%s", err, payload)
		}
		chunks = append(chunks, chunk)
	}
	return chunks, gotDone
}

// collectParts 把所有 chunk 的 parts 按顺序摊平
func collectParts(chunks []dto.GeminiChatResponse) []dto.GeminiPart {
	var parts []dto.GeminiPart
	for _, c := range chunks {
		for _, cand := range c.Candidates {
			if cand.Content != nil {
				parts = append(parts, cand.Content.Parts...)
			}
		}
	}
	return parts
}

// ===== 非流式 =====

// TestGetRequestURL_GeminiChatMode Gemini 入站（RelayModeGeminiChat）同样打 /v1/messages
// （请求侧已由 relaykit 转为 Claude Messages 格式，响应侧转回 Gemini）。
// 回归：URL switch 漏加该模式时 DoRequest 直接报 unsupported relay mode。
func TestGetRequestURL_GeminiChatMode(t *testing.T) {
	a := &Adaptor{}
	got, err := a.GetRequestURL(geminiInboundInfo(true))
	if err != nil {
		t.Fatalf("GetRequestURL(GeminiChat) error: %v", err)
	}
	if want := "https://upstream.example.com/v1/messages"; got != want {
		t.Errorf("GetRequestURL(GeminiChat) = %q, want %q", got, want)
	}
}

// TestHandleNonStreamToGemini_WritesGeminiBody 正常路径：响应体为 Gemini 结构，
// 计费用量仍为 Claude 口径（input 不含缓存）。
func TestHandleNonStreamToGemini_WritesGeminiBody(t *testing.T) {
	body := `{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5",` +
		`"content":[{"type":"text","text":"你好"}],"stop_reason":"end_turn",` +
		`"usage":{"input_tokens":40,"output_tokens":20,"cache_read_input_tokens":10,"cache_creation_input_tokens":5}}`

	a := &Adaptor{}
	info := geminiInboundInfo(false)
	rec := httptest.NewRecorder()

	usage, err := a.handleNonStreamToGemini(context.Background(), geminiJSONResponse(http.StatusOK, body), info, rec)
	if err != nil {
		t.Fatalf("handleNonStreamToGemini error: %v", err)
	}

	var got dto.GeminiChatResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("响应体不是合法 Gemini JSON: %v\nbody=%s", err, rec.Body.String())
	}
	if len(got.Candidates) != 1 || got.Candidates[0].Content == nil ||
		len(got.Candidates[0].Content.Parts) != 1 || got.Candidates[0].Content.Parts[0].Text != "你好" {
		t.Errorf("内容不对: %+v", got.Candidates)
	}
	if got.Candidates[0].Content.Role != "model" {
		t.Errorf("role = %q, want model", got.Candidates[0].Content.Role)
	}
	// 客户端可见用量为 Gemini 口径（prompt 含缓存）
	if got.UsageMetadata.PromptTokenCount != 55 {
		t.Errorf("客户端 promptTokenCount = %d, want 55", got.UsageMetadata.PromptTokenCount)
	}
	// 计费用量为 Claude 口径（prompt 不含缓存，cache_creation 独立）
	if usage.PromptTokens != 40 {
		t.Errorf("计费 PromptTokens = %d, want 40（Claude 口径不含缓存）", usage.PromptTokens)
	}
	if usage.CacheCreationTokens != 5 {
		t.Errorf("计费 CacheCreationTokens = %d, want 5", usage.CacheCreationTokens)
	}
}

// TestHandleNonStreamToGemini_UpstreamErrorNotWritten 上游错误不在本层写响应，
// 交上层 WriteGeminiRelayError 统一写入。
func TestHandleNonStreamToGemini_UpstreamErrorNotWritten(t *testing.T) {
	a := &Adaptor{}
	rec := httptest.NewRecorder()
	body := `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`

	_, err := a.handleNonStreamToGemini(context.Background(), geminiJSONResponse(http.StatusTooManyRequests, body), geminiInboundInfo(false), rec)

	if err == nil {
		t.Fatal("上游 429 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}

// ===== 流式 =====

// TestHandleStreamToGemini_TextAndThinking 思考与正文分别映射为 thought part / 普通 part，
// 收尾 chunk 带 finishReason 与 usageMetadata，最后发 [DONE]。
func TestHandleStreamToGemini_TextAndThinking(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"usage":{"input_tokens":40,"output_tokens":0,"cache_read_input_tokens":10}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"想一下"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-xyz"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		``,
		`data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"你好"}}`,
		``,
		`data: {"type":"content_block_stop","index":1}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":40,"output_tokens":20,"cache_read_input_tokens":10}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
		``,
	}, "\n")

	a := &Adaptor{}
	info := geminiInboundInfo(true)
	rec := httptest.NewRecorder()

	usage, err := a.handleStreamToGemini(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleStreamToGemini error: %v", err)
	}

	chunks, gotDone := parseGeminiSSE(t, rec.Body.String())
	if gotDone {
		t.Error("Gemini 客户端不得收到 [DONE]（真实 Gemini API 无此哨兵，官方 SDK 对 data 帧做 JSON.parse 会崩溃）")
	}
	if len(chunks) < 2 {
		t.Fatalf("chunk 数过少: %d", len(chunks))
	}
	// 收尾 chunk 的 content 不得带 "parts": null（官方 SDK isValidContent 只判 undefined
	// 后直接读 .length，null 会崩溃）；键缺失或空数组均可
	if strings.Contains(rec.Body.String(), `"parts":null`) {
		t.Error(`响应中出现 "parts":null，Gemini 官方 SDK 会崩溃`)
	}

	parts := collectParts(chunks)
	if len(parts) != 3 {
		t.Fatalf("parts 数 = %d, want 3（thinking / signature / text）: %+v", len(parts), parts)
	}
	if parts[0].Text != "想一下" || parts[0].Thought == nil || !*parts[0].Thought {
		t.Errorf("思考 part 不对: %+v", parts[0])
	}
	if parts[1].ThoughtSignature != "sig-xyz" {
		t.Errorf("signature part 不对: %+v", parts[1])
	}
	if parts[2].Text != "你好" || parts[2].Thought != nil {
		t.Errorf("文本 part 不对: %+v", parts[2])
	}

	final := chunks[len(chunks)-1]
	if final.Candidates[0].FinishReason != common.GeminiSTOP {
		t.Errorf("收尾 finishReason = %q, want STOP", final.Candidates[0].FinishReason)
	}
	if final.UsageMetadata == nil || final.UsageMetadata.PromptTokenCount != 50 {
		t.Errorf("收尾 usageMetadata 不对（应为 40+10 缓存）: %+v", final.UsageMetadata)
	}

	// 计费口径：Claude 的 input_tokens 不含缓存
	if usage.PromptTokens != 40 || usage.CompletionTokens != 20 {
		t.Errorf("计费用量 = %d/%d, want 40/20", usage.PromptTokens, usage.CompletionTokens)
	}
}

// TestHandleStreamToGemini_ToolCallBuffered Claude 的 input_json_delta 分片必须
// 缓冲拼接成完整对象后才能作为 functionCall.args 发出（Gemini 不接受分片）。
func TestHandleStreamToGemini_ToolCallBuffered(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-sonnet-5","content":[],"usage":{"input_tokens":10,"output_tokens":0}}}`,
		``,
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_7","name":"get_weather","input":{}}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"\"北京\"}"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"input_tokens":10,"output_tokens":6}}`,
		``,
		`data: {"type":"message_stop"}`,
		``,
		``,
	}, "\n")

	a := &Adaptor{}
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToGemini(context.Background(), sseResponse(strings.NewReader(upstream)), geminiInboundInfo(true), rec)
	if err != nil {
		t.Fatalf("handleStreamToGemini error: %v", err)
	}

	chunks, _ := parseGeminiSSE(t, rec.Body.String())
	parts := collectParts(chunks)
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("应恰好产出一个 functionCall part: %+v", parts)
	}

	fc := parts[0].FunctionCall
	if fc.FunctionName != "get_weather" || fc.ID != "toolu_7" {
		t.Errorf("functionCall 元信息不对: %+v", fc)
	}
	args, ok := fc.Arguments.(map[string]any)
	if !ok {
		t.Fatalf("args 应为拼接后的对象, got %T: %v", fc.Arguments, fc.Arguments)
	}
	if args["city"] != "北京" {
		t.Errorf("分片参数拼接错误: %+v", args)
	}
}

// TestHandleStreamToGemini_MalformedToolArgs 分片拼接后仍非法时降级为空对象，
// 保留函数名而不是整段丢弃或输出非法 JSON。
func TestHandleStreamToGemini_MalformedToolArgs(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"broken"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":"}}`,
		``,
		`data: {"type":"content_block_stop","index":0}`,
		``,
		``,
	}, "\n")

	a := &Adaptor{}
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToGemini(context.Background(), sseResponse(strings.NewReader(upstream)), geminiInboundInfo(true), rec)
	if err != nil {
		t.Fatalf("handleStreamToGemini error: %v", err)
	}

	chunks, _ := parseGeminiSSE(t, rec.Body.String())
	parts := collectParts(chunks)
	if len(parts) != 1 || parts[0].FunctionCall == nil {
		t.Fatalf("应产出 functionCall part: %+v", parts)
	}
	if parts[0].FunctionCall.FunctionName != "broken" {
		t.Errorf("函数名应保留: %+v", parts[0].FunctionCall)
	}
	if _, ok := parts[0].FunctionCall.Arguments.(map[string]any); !ok {
		t.Errorf("非法参数应降级为空对象: %v", parts[0].FunctionCall.Arguments)
	}
}

// TestHandleStreamToGemini_UnclosedToolFlushed 上游异常断流、工具块未收到
// content_block_stop 时兜底冲刷，避免整个调用丢失。
func TestHandleStreamToGemini_UnclosedToolFlushed(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"t1","name":"half"}}`,
		``,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"a\":1}"}}`,
		``,
		``,
	}, "\n")

	a := &Adaptor{}
	rec := httptest.NewRecorder()

	_, err := a.handleStreamToGemini(context.Background(), sseResponse(strings.NewReader(upstream)), geminiInboundInfo(true), rec)
	if err != nil {
		t.Fatalf("handleStreamToGemini error: %v", err)
	}

	parts := collectParts(mustChunks(t, rec.Body.String()))
	if len(parts) != 1 || parts[0].FunctionCall == nil || parts[0].FunctionCall.FunctionName != "half" {
		t.Fatalf("未闭合的工具块应被兜底冲刷: %+v", parts)
	}
}

func mustChunks(t *testing.T, raw string) []dto.GeminiChatResponse {
	t.Helper()
	chunks, _ := parseGeminiSSE(t, raw)
	return chunks
}

// TestHandleStreamToGemini_UpstreamErrorNotWritten 流式上游错误同样不在本层写响应
func TestHandleStreamToGemini_UpstreamErrorNotWritten(t *testing.T) {
	a := &Adaptor{}
	rec := httptest.NewRecorder()
	resp := geminiJSONResponse(http.StatusServiceUnavailable, `{"type":"error","error":{"message":"down"}}`)

	_, err := a.handleStreamToGemini(context.Background(), resp, geminiInboundInfo(true), rec)

	if err == nil {
		t.Fatal("上游 503 应返回错误")
	}
	if rec.Body.Len() != 0 {
		t.Errorf("本层不应写响应体, got %q", rec.Body.String())
	}
}
