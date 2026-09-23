package claude

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// nativeStreamInfo 构造 Claude 原生透传（/v1/messages 直通）的 RelayInfo
func nativeStreamInfo() *common.RelayInfo {
	return &common.RelayInfo{
		RelayMode:       int(constant.RelayModeClaudeMessages),
		IsStream:        true,
		InboundFormat:   constant.RelayFormatClaude,
		ClientFormat:    constant.RelayFormatClaude,
		OriginModelName: "claude-opus-4-8",
		StartTime:       time.Now(),
		StreamStatus:    common.NewStreamStatus(),
		ChannelMeta: &common.ChannelMeta{
			BaseURL:           "https://upstream.example.com",
			UpstreamModelName: "claude-opus-4-8",
			IsModelMapped:     false,
		},
	}
}

func sseResponse(body io.Reader) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(body),
	}
}

// TestHandleClaudeNativeStream_FramesForwardedIntact 正常路径：上游 SSE 逐字节原样转发，
// 每个事件都带完整 data（客户端解码后不会出现空 data）。
func TestHandleClaudeNativeStream_FramesForwardedIntact(t *testing.T) {
	upstream := strings.Join([]string{
		`event: message_start`,
		`data: {"type":"message_start","message":{"id":"msg_01","model":"claude-opus-4-8","usage":{"input_tokens":2,"output_tokens":0,"cache_creation_input_tokens":5161}}}`,
		``,
		`event: content_block_delta`,
		`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"你好"}}`,
		``,
		`event: message_delta`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":7}}`,
		``,
		`event: message_stop`,
		`data: {"type":"message_stop"}`,
		``,
		``,
	}, "\n")

	a := &Adaptor{}
	info := nativeStreamInfo()
	rec := httptest.NewRecorder()

	usage, err := a.handleClaudeNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec)
	if err != nil {
		t.Fatalf("handleClaudeNativeStream error: %v", err)
	}

	if got := rec.Body.String(); got != upstream {
		t.Errorf("转发内容与上游不一致:\ngot:  %q\nwant: %q", got, upstream)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonDone {
		t.Errorf("EndReason = %q, want %q", reason, common.StreamEndReasonDone)
	}
	if usage == nil || usage.PromptTokens != 2 || usage.CompletionTokens != 7 || usage.CacheCreationTokens != 5161 {
		t.Errorf("usage = %+v, want prompt=2 completion=7 cacheCreation=5161", usage)
	}
}

// TestHandleClaudeNativeStream_LastLineWithoutNewline 上游最后一行没有换行结尾时
// 不得丢弃 —— ReadString 会同时返回「残留数据 + io.EOF」，旧实现直接 break 会吞掉这一行。
func TestHandleClaudeNativeStream_LastLineWithoutNewline(t *testing.T) {
	upstream := "event: message_stop\ndata: {\"type\":\"message_stop\"}"

	a := &Adaptor{}
	info := nativeStreamInfo()
	rec := httptest.NewRecorder()

	if _, err := a.handleClaudeNativeStream(context.Background(), sseResponse(strings.NewReader(upstream)), info, rec); err != nil {
		t.Fatalf("handleClaudeNativeStream error: %v", err)
	}

	if got := rec.Body.String(); got != upstream {
		t.Errorf("无换行结尾的最后一行被丢弃:\ngot:  %q\nwant: %q", got, upstream)
	}
}

// TestHandleClaudeNativeStream_ClientGoneDiscardsPartialFrame 流中断时，
// 尚未攒齐的半帧必须丢弃，客户端只应看到完整的 error 帧 —— 否则半帧与 error 帧
// 拼在一起会变成非法 SSE，客户端反而先在 JSON 解析上炸掉。
func TestHandleClaudeNativeStream_ClientGoneDiscardsPartialFrame(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 吐出一个完整帧后取消 ctx，再交出半帧（只有 event 行）：主循环把半帧收进缓冲、
	// 回到循环顶部时 select 命中 Done，模拟客户端在帧中途断开
	halfSent := false
	body := io.MultiReader(
		strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2,\"cache_creation_input_tokens\":5161}}}\n\n"),
		readerFunc(func(p []byte) (int, error) {
			if halfSent {
				return 0, io.EOF
			}
			halfSent = true
			cancel()
			return copy(p, "event: content_block_delta\n"), nil
		}),
	)

	a := &Adaptor{}
	info := nativeStreamInfo()
	rec := httptest.NewRecorder()

	usage, err := a.handleClaudeNativeStream(ctx, sseResponse(body), info, rec)
	if !errors.Is(err, common.ErrStreamInterrupted) {
		t.Fatalf("err = %v, want ErrStreamInterrupted", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonClientGone {
		t.Errorf("EndReason = %q, want %q", reason, common.StreamEndReasonClientGone)
	}

	got := rec.Body.String()
	if strings.Contains(got, "content_block_delta") {
		t.Errorf("半帧未被丢弃，泄漏到客户端:\n%q", got)
	}
	if !strings.Contains(got, `"type":"error"`) {
		t.Errorf("未写出 error 帧，客户端会挂起:\n%q", got)
	}
	// 中断兜底仍要保留已拿到的 usage，避免按 0 token 结算
	if usage == nil || usage.PromptTokens != 2 || usage.CacheCreationTokens != 5161 {
		t.Errorf("usage = %+v, want prompt=2 cacheCreation=5161", usage)
	}
}

// TestHandleClaudeNativeStream_UpstreamErrorDiscardsPartialFrame 上游读取出错时，
// 缓冲里的残留是不完整的帧，不得写给客户端。
func TestHandleClaudeNativeStream_UpstreamErrorDiscardsPartialFrame(t *testing.T) {
	upstreamErr := errors.New("unexpected EOF from upstream")
	body := io.MultiReader(
		strings.NewReader("event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: content_block_delta\n"),
		readerFunc(func(p []byte) (int, error) { return 0, upstreamErr }),
	)

	a := &Adaptor{}
	info := nativeStreamInfo()
	rec := httptest.NewRecorder()

	_, err := a.handleClaudeNativeStream(context.Background(), sseResponse(body), info, rec)
	if err == nil || !strings.Contains(err.Error(), "upstream stream interrupted") {
		t.Fatalf("err = %v, want upstream stream interrupted", err)
	}
	if reason := info.StreamStatus.GetEndReason(); reason != common.StreamEndReasonError {
		t.Errorf("EndReason = %q, want %q", reason, common.StreamEndReasonError)
	}
	if got := rec.Body.String(); strings.Contains(got, "content_block_delta") {
		t.Errorf("上游出错时半帧泄漏到客户端:\n%q", got)
	}
}

// readerFunc 把函数适配成 io.Reader
type readerFunc func(p []byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }
