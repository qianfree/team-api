package handler

import (
	"context"
	"strings"
	"testing"

	"github.com/qianfree/team-api/relay/common"
	"github.com/qianfree/team-api/relay/constant"
)

// captureUsageProvider 捕获 RecordUsage 调用的假 DataProvider（嵌入接口，仅覆盖本测试用到的方法）
type captureUsageProvider struct {
	common.DataProvider
	records []*common.UsageRecord
}

func (p *captureUsageProvider) RecordUsage(_ context.Context, record *common.UsageRecord) {
	p.records = append(p.records, record)
}

// TestHandleChannelUnavailableAllFailed 全部渠道失败分支：用量日志须补录最后一次实际发出
// 请求的渠道（此前固定传 nil，渠道字段全空），并把重试循环累积的转发链路挂入 rc.ForwardingTrace。
func TestHandleChannelUnavailableAllFailed(t *testing.T) {
	provider := &captureUsageProvider{}
	rc := &RelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req-all-failed"}
	v := &relayValidation{
		relayMode:     constant.RelayModeChatCompletions,
		modelName:     "gpt-test",
		isStream:      true,
		billingResult: &BillingResult{},
	}
	trace := &common.ForwardingTrace{EntryPath: "/v1/chat/completions"}
	selection := &common.ChannelSelection{
		ChannelID:         7,
		ChannelName:       "上游A",
		ChannelType:       1,
		UpstreamModelName: "gpt-upstream",
	}
	last := &lastAttemptInfo{trace: trace, selection: selection, attempt: 2}

	result := handleChannelUnavailable(
		context.Background(), nil, provider, rc, v, 0,
		[]string{
			"attempt=0 channel=5(甲) error=[timeout]",
			"attempt=1 channel=6(乙) error=[500]",
			"attempt=2 channel=7(上游A) error=[429]",
		},
		common.ErrChannelUnavailable, "熔断OPEN×3", last,
	)

	// NewChannelError 的 Error() 会拼上 cause（ErrAllChannelsFailed），校验前缀即可
	if result.err == nil || !strings.HasPrefix(result.err.Error(), "当前模型暂时不可用") {
		t.Fatalf("err = %v, want 当前模型暂时不可用前缀", result.err)
	}
	if len(provider.records) != 1 {
		t.Fatalf("RecordUsage calls = %d, want 1", len(provider.records))
	}
	rec := provider.records[0]
	if rec.Success || rec.Status != "error" {
		t.Errorf("Success/Status = %v/%s, want false/error", rec.Success, rec.Status)
	}
	if rec.ChannelID != 7 || rec.ChannelName != "上游A" || rec.ChannelType != 1 || rec.UpstreamModel != "gpt-upstream" {
		t.Errorf("channel fields = %d/%s/%d/%s, want 7/上游A/1/gpt-upstream",
			rec.ChannelID, rec.ChannelName, rec.ChannelType, rec.UpstreamModel)
	}
	if rec.RetryIndex != 2 {
		t.Errorf("RetryIndex = %d, want 2", rec.RetryIndex)
	}
	// error_message 对租户可见：保持通用文案（含 cause 拼接），不得拼入渠道明细
	if !strings.HasPrefix(rec.ErrorMessage, "当前模型暂时不可用") {
		t.Errorf("ErrorMessage = %q, want 当前模型暂时不可用前缀", rec.ErrorMessage)
	}
	if strings.Contains(rec.ErrorMessage, "channel=") || strings.Contains(rec.ErrorMessage, "上游A") {
		t.Errorf("ErrorMessage = %q, 泄漏了渠道明细", rec.ErrorMessage)
	}

	// 转发链路须挂入 rc（审计日志据此记录每跳渠道与错误），尝试数与上游模型补全
	if rc.ForwardingTrace != trace {
		t.Fatal("rc.ForwardingTrace 未挂载累积的转发链路")
	}
	if trace.TotalAttempts != 3 {
		t.Errorf("TotalAttempts = %d, want 3", trace.TotalAttempts)
	}
	if trace.UpstreamModel != "gpt-upstream" || trace.ModelMapped {
		t.Errorf("UpstreamModel/ModelMapped = %q/%v, want gpt-upstream/false", trace.UpstreamModel, trace.ModelMapped)
	}
}

// TestHandleChannelUnavailableAllFailedNoSelection 每轮尝试都在物化阶段失败（Key 解密失败等）：
// 从未实际发出上游请求，渠道字段保持留空、轮次为 0，转发链路仍须挂载（调度决策有审计价值）。
func TestHandleChannelUnavailableAllFailedNoSelection(t *testing.T) {
	provider := &captureUsageProvider{}
	rc := &RelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req-no-selection"}
	v := &relayValidation{
		relayMode:     constant.RelayModeChatCompletions,
		modelName:     "gpt-test",
		billingResult: &BillingResult{},
	}
	trace := &common.ForwardingTrace{EntryPath: "/v1/chat/completions"}
	last := &lastAttemptInfo{trace: trace}

	handleChannelUnavailable(
		context.Background(), nil, provider, rc, v, 0,
		[]string{"attempt=0 channel=5 materialize_error=[key decrypt failed]"},
		common.ErrChannelUnavailable, "", last,
	)

	if len(provider.records) != 1 {
		t.Fatalf("RecordUsage calls = %d, want 1", len(provider.records))
	}
	rec := provider.records[0]
	if rec.ChannelID != 0 || rec.ChannelName != "" || rec.ChannelType != 0 || rec.UpstreamModel != "" {
		t.Errorf("channel fields = %d/%s/%d/%s, 从未发出请求应留空",
			rec.ChannelID, rec.ChannelName, rec.ChannelType, rec.UpstreamModel)
	}
	if rec.RetryIndex != 0 {
		t.Errorf("RetryIndex = %d, want 0", rec.RetryIndex)
	}
	if rc.ForwardingTrace != trace || trace.TotalAttempts != 1 {
		t.Errorf("trace 挂载/TotalAttempts = %v/%d, want 已挂载/1", rc.ForwardingTrace == trace, trace.TotalAttempts)
	}
	if trace.UpstreamModel != "" {
		t.Errorf("UpstreamModel = %q, 无物化成功的渠道应为空", trace.UpstreamModel)
	}
}

// TestHandleChannelUnavailableNoAttempts 无可用渠道且一次尝试都没发生（未配渠道/全熔断）：
// 维持既有约定——不记用量日志（零消耗 error 记录不污染请求日志）、不挂转发链路。
func TestHandleChannelUnavailableNoAttempts(t *testing.T) {
	provider := &captureUsageProvider{}
	rc := &RelayContext{TenantID: 1, UserID: 2, ApiKeyID: 3, RequestID: "req-no-attempt"}
	v := &relayValidation{
		relayMode:     constant.RelayModeChatCompletions,
		modelName:     "gpt-test",
		billingResult: &BillingResult{},
	}
	trace := &common.ForwardingTrace{EntryPath: "/v1/chat/completions"}
	last := &lastAttemptInfo{trace: trace}

	result := handleChannelUnavailable(
		context.Background(), nil, provider, rc, v, 0,
		nil, common.ErrChannelUnavailable, "目录为空", last,
	)

	if result.err == nil {
		t.Fatal("err = nil, want 无可用渠道错误")
	}
	if len(provider.records) != 0 {
		t.Errorf("RecordUsage calls = %d, 无尝试发生不应记用量", len(provider.records))
	}
	if rc.ForwardingTrace != nil {
		t.Error("rc.ForwardingTrace 应保持 nil（无任何跳可记）")
	}
}
