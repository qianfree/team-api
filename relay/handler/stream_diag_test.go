package handler

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// TestApplyStreamEndDiag_SentinelReplacedBySummary 哨兵错误（ErrStreamInterrupted）本身无归因价值，
// error_message 必须换成 StreamStatus 摘要，让用量日志能区分 context canceled / TCP 写失败等真实原因。
func TestApplyStreamEndDiag_SentinelReplacedBySummary(t *testing.T) {
	status := common.NewStreamStatus()
	status.SetEndReason(common.StreamEndReasonClientGone, context.Canceled)

	record := &common.UsageRecord{ErrorMessage: common.ErrStreamInterrupted.Error()}
	applyStreamEndDiag(record, status, common.ErrStreamInterrupted)

	if record.StreamEndReason != string(common.StreamEndReasonClientGone) {
		t.Errorf("StreamEndReason = %q, want %q", record.StreamEndReason, common.StreamEndReasonClientGone)
	}
	want := "client_gone: context canceled"
	if record.ErrorMessage != want {
		t.Errorf("ErrorMessage = %q, want %q", record.ErrorMessage, want)
	}
}

// TestApplyStreamEndDiag_StatusCarriesErrorSummaryWins 状态本身带底层错误（如 handler_stop 的
// 写客户端失败）时摘要信息量高于外层错误，直接以摘要作为 error_message。
func TestApplyStreamEndDiag_StatusCarriesErrorSummaryWins(t *testing.T) {
	status := common.NewStreamStatus()
	writeErr := errors.New("write tcp 127.0.0.1:18888->127.0.0.1:6396: wsasend: connection forcibly closed")
	status.SetEndReason(common.StreamEndReasonHandlerStop, writeErr)

	record := &common.UsageRecord{ErrorMessage: "upstream error"}
	applyStreamEndDiag(record, status, errors.New("upstream error"))

	if !strings.Contains(record.ErrorMessage, "write tcp") {
		t.Errorf("ErrorMessage should keep underlying write error, got %q", record.ErrorMessage)
	}
	if record.StreamEndReason != string(common.StreamEndReasonHandlerStop) {
		t.Errorf("StreamEndReason = %q, want %q", record.StreamEndReason, common.StreamEndReasonHandlerStop)
	}
}

// TestApplyStreamEndDiag_RealErrorKeptWithSummary 非哨兵外层错误（如上游 5xx）且状态无底层错误时
// （如 eof），保留原始错误并把流结束原因追加在后，两条线索都不丢。
func TestApplyStreamEndDiag_RealErrorKeptWithSummary(t *testing.T) {
	status := common.NewStreamStatus()
	status.SetEndReason(common.StreamEndReasonEOF, nil)

	upstreamErr := errors.New("upstream returned HTTP 502")
	record := &common.UsageRecord{ErrorMessage: upstreamErr.Error()}
	applyStreamEndDiag(record, status, upstreamErr)

	if !strings.HasPrefix(record.ErrorMessage, upstreamErr.Error()) {
		t.Errorf("ErrorMessage should keep original error, got %q", record.ErrorMessage)
	}
	if !strings.Contains(record.ErrorMessage, string(common.StreamEndReasonEOF)) {
		t.Errorf("ErrorMessage should append stream summary, got %q", record.ErrorMessage)
	}
	if record.StreamEndReason != string(common.StreamEndReasonEOF) {
		t.Errorf("StreamEndReason = %q, want %q", record.StreamEndReason, common.StreamEndReasonEOF)
	}
}

// TestApplyStreamEndDiag_NoopCases nil 状态与未设置结束原因的状态不得改动记录。
func TestApplyStreamEndDiag_NoopCases(t *testing.T) {
	record := &common.UsageRecord{ErrorMessage: "raw"}
	applyStreamEndDiag(record, nil, errors.New("x"))
	if record.StreamEndReason != "" || record.ErrorMessage != "raw" {
		t.Errorf("nil status should be no-op, got reason=%q message=%q", record.StreamEndReason, record.ErrorMessage)
	}

	empty := common.NewStreamStatus()
	applyStreamEndDiag(record, empty, errors.New("x"))
	if record.StreamEndReason != "" || record.ErrorMessage != "raw" {
		t.Errorf("empty reason should be no-op, got reason=%q message=%q", record.StreamEndReason, record.ErrorMessage)
	}
}

// TestApplyStreamEndDiag_UpstreamURLNeverLeaks 用量日志 error_message 对租户可见，
// 编排层包装的上游传输错误（*url.Error 含完整上游域名/IP）必须归一化，禁止原样落库。
func TestApplyStreamEndDiag_UpstreamURLNeverLeaks(t *testing.T) {
	// 还原线上泄露形态：NewUpstreamError(502, "请求处理失败", Post "https://api.deepseek.com/...": dial tcp ...)
	dialErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connectex: A connection attempt failed because the connected party did not properly respond"),
	}
	upstreamErr := &url.Error{Op: "Post", URL: "https://api.deepseek.com/anthropic/v1/messages", Err: dialErr}
	relayErr := constant.NewUpstreamError(502, "请求处理失败", upstreamErr)

	status := common.NewStreamStatus()
	status.SetEndReason(common.StreamEndReasonEOF, nil)

	record := &common.UsageRecord{ErrorMessage: relayErr.Error()}
	applyStreamEndDiag(record, status, relayErr)

	for _, leak := range []string{"api.deepseek.com", "171.105.220.186", "https://", "/anthropic/"} {
		if strings.Contains(record.ErrorMessage, leak) {
			t.Errorf("error_message 泄露上游信息 %q: %q", leak, record.ErrorMessage)
		}
	}
}

// TestApplyStreamEndDiag_SummaryLocalAddrRedacted 摘要中的写客户端 TCP 错误含本地地址，
// IP 必须抹除（保留 wsasend 等错误类别供排查）。
func TestApplyStreamEndDiag_SummaryLocalAddrRedacted(t *testing.T) {
	status := common.NewStreamStatus()
	status.SetEndReason(common.StreamEndReasonHandlerStop,
		errors.New("write tcp 127.0.0.1:18888->127.0.0.1:6396: wsasend: An existing connection was forcibly closed"))

	record := &common.UsageRecord{ErrorMessage: "x"}
	applyStreamEndDiag(record, status, common.ErrStreamInterrupted)

	if strings.Contains(record.ErrorMessage, "127.0.0.1") {
		t.Errorf("摘要未抹除本地 IP: %q", record.ErrorMessage)
	}
	if !strings.Contains(record.ErrorMessage, "wsasend") {
		t.Errorf("应保留错误类别 wsasend: %q", record.ErrorMessage)
	}
	if !strings.Contains(record.ErrorMessage, string(common.StreamEndReasonHandlerStop)) {
		t.Errorf("应保留结束原因 handler_stop: %q", record.ErrorMessage)
	}
}
