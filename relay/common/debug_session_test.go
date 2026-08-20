package common

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/constant"
	"github.com/qianfree/team-api/relaykit/types"
)

// 验证调试会话完整生命周期：
// 中间失败尝试 Submit 立即提交（is_final=false、段4为空），
// 最终尝试 MarkFinal 挂起、FinalizeAndSubmit 补段4后提交，且幂等。
func TestDebugSessionLifecycle(t *testing.T) {
	var records []*DebugLogRecord
	origHook := SubmitDebugLog
	SubmitDebugLog = func(ctx context.Context, r *DebugLogRecord) { records = append(records, r) }
	defer func() { SubmitDebugLog = origHook }()

	s := NewDebugSession("req-1", 1, 2, 3, "/v1/chat/completions")
	// 段1：客户端请求（含凭证头，应脱敏）
	h := http.Header{}
	h.Set("Authorization", "Bearer sk-test-key-12345678")
	h.Set("Content-Type", "application/json")
	s.CaptureClientRequest(h, []byte(`{"model":"m"}`))

	selection := &ChannelSelection{ChannelID: 9, ChannelName: "ch9", ChannelType: 1, UpstreamModelName: "up-m"}

	// 第一次尝试失败（中间）：Submit 立即提交
	a1 := s.BeginAttempt(selection, "m", "chat_completions", true, 0)
	a1.Submit(errors.New("boom"))
	if len(records) != 1 {
		t.Fatalf("中间尝试应提交 1 条，实际 %d", len(records))
	}
	r1 := records[0]
	if r1.IsFinal {
		t.Error("中间尝试 is_final 应为 false")
	}
	if r1.Error != "boom" || r1.ChannelID != 9 || r1.RetryIndex != 0 || !r1.IsStream {
		t.Errorf("中间尝试元数据错误: %+v", r1)
	}
	if string(r1.ClientReqBody) != `{"model":"m"}` {
		t.Errorf("段1 请求体错误: %q", r1.ClientReqBody)
	}
	if r1.ClientReqHeaders["Authorization"] != "Bearer****5678" {
		t.Errorf("段1 凭证头未脱敏: %q", r1.ClientReqHeaders["Authorization"])
	}
	if r1.ClientRespBody != nil {
		t.Errorf("中间尝试段4应为空: %q", r1.ClientRespBody)
	}

	// 第二次尝试成功：MarkFinal → 模拟段2捕获 → FinalizeAndSubmit 补段4
	a2 := s.BeginAttempt(selection, "m", "chat_completions", true, 1)
	req, _ := http.NewRequest(http.MethodPost, "http://upstream/v1/chat", strings.NewReader("converted-body"))
	a2.Capture.captureUpstreamRequest(req)
	// 模拟传输层读取请求体 + 上游响应
	_, _ = io.ReadAll(req.Body)
	_ = req.Body.Close()
	resp := &http.Response{StatusCode: 200, Header: http.Header{"X-Up": []string{"v"}}, Body: io.NopCloser(strings.NewReader("upstream-resp"))}
	a2.Capture.captureUpstreamResponse(resp)
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	// 段4 writer
	rec := httptest.NewRecorder()
	dw := NewDebugClientWriter(rec)
	s.SetClientWriter(dw)
	dw.WriteHeader(200)
	_, _ = dw.Write([]byte("client-resp"))

	a2.MarkFinal(nil)
	s.FinalizeAndSubmit(123, 45)

	if len(records) != 2 {
		t.Fatalf("最终尝试应提交第 2 条，实际 %d", len(records))
	}
	r2 := records[1]
	if !r2.IsFinal || r2.RetryIndex != 1 || r2.Error != "" {
		t.Errorf("最终尝试元数据错误: %+v", r2)
	}
	if string(r2.UpstreamReqBody) != "converted-body" {
		t.Errorf("段2 请求体错误: %q", r2.UpstreamReqBody)
	}
	if r2.UpstreamStatusCode != 200 || r2.UpstreamRespHeaders["X-Up"] != "v" {
		t.Errorf("段3 状态/响应头错误: %d %v", r2.UpstreamStatusCode, r2.UpstreamRespHeaders)
	}
	if string(r2.UpstreamRespBody) != "upstream-resp" {
		t.Errorf("段3 响应体错误: %q", r2.UpstreamRespBody)
	}
	if string(r2.ClientRespBody) != "client-resp" || r2.ClientStatusCode != 200 {
		t.Errorf("段4 错误: %q %d", r2.ClientRespBody, r2.ClientStatusCode)
	}
	if r2.TotalLatencyMs != 123 || r2.FirstTokenMs != 45 {
		t.Errorf("耗时元数据错误: %d %d", r2.TotalLatencyMs, r2.FirstTokenMs)
	}

	// 幂等：重复 Finalize 不再提交
	s.FinalizeAndSubmit(1, 1)
	if len(records) != 2 {
		t.Errorf("FinalizeAndSubmit 应幂等，实际提交 %d 条", len(records))
	}
}

// nil 会话（开关未开启）调用 FinalizeAndSubmit 应安全 no-op
func TestDebugSessionNilSafe(t *testing.T) {
	var s *DebugSession
	s.FinalizeAndSubmit(0, 0) // 不应 panic

	var a *DebugAttempt
	a.Submit(errors.New("x")) // 不应 panic
	a.MarkFinal(nil)          // 不应 panic
}

// 调试目标过滤：AND 组合，0 = 不限
func TestDebugTargetMatch(t *testing.T) {
	s := ChannelSettings{DebugLogEnabled: true}
	if !s.DebugTargetMatch(1, 2, 3) {
		t.Error("未设过滤器应全部匹配")
	}

	s.DebugLogTenantID = 10
	if !s.DebugTargetMatch(10, 99, 99) {
		t.Error("匹配租户时应通过")
	}
	if s.DebugTargetMatch(11, 2, 3) {
		t.Error("租户不匹配应拒绝")
	}

	s.DebugLogUserID = 20
	if !s.DebugTargetMatch(10, 20, 99) {
		t.Error("租户+成员均匹配应通过")
	}
	if s.DebugTargetMatch(10, 21, 99) {
		t.Error("成员不匹配应拒绝")
	}

	s.DebugLogApiKeyID = 30
	if !s.DebugTargetMatch(10, 20, 30) {
		t.Error("三者均匹配应通过")
	}
	if s.DebugTargetMatch(10, 20, 31) {
		t.Error("密钥不匹配应拒绝")
	}
}

// 协议转换快照：常规转换按渠道类型推导上游协议；桥接标志优先；链路兜底两端
func TestCaptureProtocol(t *testing.T) {
	s := NewDebugSession("req", 1, 2, 3, "/v1/chat/completions")
	selection := &ChannelSelection{ChannelID: 9, ChannelType: int(constant.ProviderClaude)}
	a := s.BeginAttempt(selection, "m", "chat_completions", false, 0)

	// 客户端 openai → claude 渠道：上游协议 claude，链路兜底两端
	info := &RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta:   &ChannelMeta{ChannelType: int(constant.ProviderClaude)},
	}
	info.AppendRequestConversion(types.RelayFormat(constant.RelayFormatOpenAI))
	a.CaptureProtocol(info)
	if a.conversion == nil {
		t.Fatal("conversion 未快照")
	}
	if a.conversion.ClientFormat != "openai" || a.conversion.UpstreamFormat != "claude" {
		t.Errorf("协议推导错误: %+v", a.conversion)
	}
	if a.conversion.Bridge != "" {
		t.Errorf("常规转换不应有桥接标志: %q", a.conversion.Bridge)
	}

	// 直传：上游协议 = 客户端协议
	info2 := &RelayInfo{
		InboundFormat: constant.RelayFormatOpenAI,
		ChannelMeta: &ChannelMeta{
			ChannelType: int(constant.ProviderOpenAI),
			Settings:    ChannelSettings{PassThroughBodyEnabled: true},
		},
	}
	a2 := s.BeginAttempt(selection, "m", "chat_completions", false, 1)
	a2.CaptureProtocol(info2)
	if a2.conversion.UpstreamFormat != "openai" || a2.conversion.Bridge != "pass_through" {
		t.Errorf("直传推导错误: %+v", a2.conversion)
	}

	// Responses 桥接
	info3 := &RelayInfo{
		InboundFormat:   constant.RelayFormatOpenAI,
		UseResponsesAPI: true,
		ChannelMeta:     &ChannelMeta{ChannelType: int(constant.ProviderOpenAI)},
	}
	a3 := s.BeginAttempt(selection, "m", "chat_completions", false, 2)
	a3.CaptureProtocol(info3)
	if a3.conversion.UpstreamFormat != "responses" || a3.conversion.Bridge != "responses_api" {
		t.Errorf("Responses 桥接推导错误: %+v", a3.conversion)
	}
}
