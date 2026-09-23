package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relay/dto"
)

// 本文件覆盖两条改用池化帧写入器（helper.WriteSSEDataJSON）的转换路径：
//   - handleGeminiInboundStream：OpenAI 上游 → Gemini 客户端
//   - HandleResponsesStreamToChat：Responses 上游 → Chat Completions 客户端
//
// 两条路径此前无测试覆盖。断言两件事：
//  1. 输出仍是合法且内容正确的 SSE（池化不能改变对外协议）；
//  2. 每次 Write 都落在帧边界（整帧单次写出，并发保活 ping 插不进帧中间）。

// assertFramesAreSingleWrites 断言每次 Write 都是一个以空行收尾的完整帧
func assertFramesAreSingleWrites(t *testing.T, chunks []string) {
	t.Helper()
	if len(chunks) == 0 {
		t.Fatal("没有任何写出")
	}
	for i, c := range chunks {
		if !strings.HasSuffix(c, "\n\n") {
			t.Errorf("第 %d 次 Write 未落在帧边界，保活 ping 可插入其中:\n%q", i, c)
		}
	}
}

// collectSSEDataFrames 从 SSE 文本中取出所有 data 行的内容
func collectSSEDataFrames(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "data:") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return out
}

func geminiInboundInfo() *common.RelayInfo {
	return &common.RelayInfo{
		IsStream:        true,
		InboundFormat:   constant.RelayFormatGemini,
		ClientFormat:    constant.RelayFormatGemini,
		OriginModelName: "gemini-3-pro",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gpt-4o",
		},
	}
}

// TestHandleGeminiInboundStream_PooledFramesValid OpenAI SSE → Gemini SSE：
// 每个 chunk 都是完整可解析的 Gemini JSON，且整帧单次写出。
func TestHandleGeminiInboundStream_PooledFramesValid(t *testing.T) {
	upstream := strings.Join([]string{
		`data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"你好"}}]}`,
		``,
		`data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"世界"}}]}`,
		``,
		`data: {"id":"c1","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":11,"completion_tokens":7,"total_tokens":18}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstream)),
	}

	info := geminiInboundInfo()
	rec := &sseFrameRecorder{}

	usage, err := handleGeminiInboundStream(context.Background(), resp, info, rec)
	if err != nil {
		t.Fatalf("handleGeminiInboundStream error: %v", err)
	}

	assertFramesAreSingleWrites(t, rec.chunks)

	frames := collectSSEDataFrames(rec.body.String())
	if len(frames) < 2 {
		t.Fatalf("data 帧数 = %d, 期望至少 2（内容 chunk + 终止 chunk）\n%s", len(frames), rec.body.String())
	}

	var text strings.Builder
	for i, f := range frames {
		if f == "[DONE]" {
			// 真实 Gemini API 无 [DONE] 哨兵；官方 SDK 对每个 data 帧做 JSON.parse，
			// 写 [DONE] 会直接抛 SyntaxError 导致客户端报错
			t.Errorf("Gemini 客户端不得收到 [DONE]（第 %d 帧）", i)
			continue
		}
		var chunk dto.GeminiChatResponse
		if err := json.Unmarshal([]byte(f), &chunk); err != nil {
			t.Fatalf("第 %d 帧不是合法 Gemini JSON（池化若出错会产生这种残片）: %v\ndata=%q", i, err, f)
		}
		for _, cand := range chunk.Candidates {
			if cand.Content == nil {
				continue
			}
			for _, p := range cand.Content.Parts {
				text.WriteString(p.Text)
			}
		}
	}
	// 收尾 chunk 的 content 不得带 "parts": null（官方 SDK isValidContent 只判 undefined
	// 后直接读 .length，null 会崩溃）
	if strings.Contains(rec.body.String(), `"parts":null`) {
		t.Error(`响应中出现 "parts":null，Gemini 官方 SDK 会崩溃`)
	}
	if got := text.String(); got != "你好世界" {
		t.Errorf("转发文本 = %q, want %q", got, "你好世界")
	}
	if usage == nil || usage.PromptTokens != 11 || usage.CompletionTokens != 7 {
		t.Errorf("usage = %+v, want prompt=11 completion=7", usage)
	}
}

func responsesToChatInfo() *common.RelayInfo {
	return &common.RelayInfo{
		IsStream:        true,
		InboundFormat:   constant.RelayFormatOpenAI,
		ClientFormat:    constant.RelayFormatOpenAI,
		OriginModelName: "gpt-4o",
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			ChannelType:       int(constant.ProviderOpenAI),
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "gpt-4o",
			SupportsResponses: true,
		},
	}
}

// TestHandleResponsesStreamToChat_PooledFramesValid Responses SSE → Chat SSE：
// 每个 chunk 都是完整可解析的 chat.completion.chunk，且整帧单次写出。
func TestHandleResponsesStreamToChat_PooledFramesValid(t *testing.T) {
	upstream := strings.Join([]string{
		"event: response.created",
		`data: {"type":"response.created","response":{"id":"resp_1","object":"response","status":"in_progress"}}`,
		``,
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"你好"}`,
		``,
		"event: response.output_text.delta",
		`data: {"type":"response.output_text.delta","delta":"世界"}`,
		``,
		"event: response.completed",
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(upstream)),
	}

	info := responsesToChatInfo()
	rec := &sseFrameRecorder{}

	if _, err := HandleResponsesStreamToChat(context.Background(), resp, info, rec); err != nil {
		t.Fatalf("HandleResponsesStreamToChat error: %v", err)
	}

	assertFramesAreSingleWrites(t, rec.chunks)

	frames := collectSSEDataFrames(rec.body.String())
	var text strings.Builder
	sawDone := false
	for i, f := range frames {
		if f == "[DONE]" {
			sawDone = true
			continue
		}
		var chunk dto.ChatCompletionStreamResponse
		if err := json.Unmarshal([]byte(f), &chunk); err != nil {
			t.Fatalf("第 %d 帧不是合法 chat.completion.chunk: %v\ndata=%q", i, err, f)
		}
		if chunk.Object != "chat.completion.chunk" {
			t.Errorf("第 %d 帧 object = %q, want chat.completion.chunk", i, chunk.Object)
		}
		for _, c := range chunk.Choices {
			if s, ok := c.Delta.Content.(string); ok {
				text.WriteString(s)
			}
		}
	}
	if !sawDone {
		t.Error("缺少 [DONE] 终止帧")
	}
	if got := text.String(); got != "你好世界" {
		t.Errorf("转发文本 = %q, want %q", got, "你好世界")
	}
}
