package helper

import (
	"errors"
	"testing"

	"github.com/qianfree/team-api/relay/common"
)

// newInterruptedInfo 构造一个已标记为客户端断开（部分传输）的 RelayInfo
func newInterruptedInfo(estimatePromptTokens int) *common.RelayInfo {
	info := &common.RelayInfo{StreamStatus: common.NewStreamStatus()}
	info.StreamStatus.SetEndReason(common.StreamEndReasonClientGone, errors.New("client gone"))
	info.SetEstimatePromptTokens(estimatePromptTokens)
	return info
}

// newNormalEndInfo 构造一个正常结束的 RelayInfo
func newNormalEndInfo() *common.RelayInfo {
	info := &common.RelayInfo{StreamStatus: common.NewStreamStatus()}
	info.StreamStatus.SetEndReason(common.StreamEndReasonDone, nil)
	return info
}

func TestEstimateStreamOutputTokens(t *testing.T) {
	tests := []struct {
		name    string
		info    *common.RelayInfo
		textLen int
		want    int
	}{
		{"空文本返回 0", newNormalEndInfo(), 0, 0},
		{"nil info 按正常口径 4 字符/token", nil, 100, 25},
		{"正常结束 4 字符/token", newNormalEndInfo(), 100, 25},
		{"流中断 2 字符/token", newInterruptedInfo(0), 100, 50},
		{"流中断奇数长度向上取整", newInterruptedInfo(0), 101, 51},
		{"无 StreamStatus 按正常口径", &common.RelayInfo{}, 100, 25},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateStreamOutputTokens(tt.info, tt.textLen); got != tt.want {
				t.Errorf("EstimateStreamOutputTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestApplyInterruptedUsageFallback_NormalEndNoOp(t *testing.T) {
	info := newNormalEndInfo()
	usage := &common.Usage{}
	ApplyInterruptedUsageFallback(info, usage, 1000)
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.TotalTokens != 0 {
		t.Errorf("正常结束不应修改 usage，got %+v", usage)
	}
}

func TestApplyInterruptedUsageFallback_FillsPromptAndOutput(t *testing.T) {
	// 中断且上游 usage 完全缺失：输入用请求侧估算补齐，输出按 2 字符/token
	info := newInterruptedInfo(120)
	usage := &common.Usage{}
	ApplyInterruptedUsageFallback(info, usage, 200)
	if usage.PromptTokens != 120 {
		t.Errorf("PromptTokens = %d, want 120（请求侧估算值）", usage.PromptTokens)
	}
	if usage.CompletionTokens != 100 {
		t.Errorf("CompletionTokens = %d, want 100（200 字符 / 2）", usage.CompletionTokens)
	}
	if usage.TotalTokens != 220 {
		t.Errorf("TotalTokens = %d, want 220", usage.TotalTokens)
	}
}

func TestApplyInterruptedUsageFallback_KeepsRealUsage(t *testing.T) {
	// 中断但已拿到真实 usage（如 Claude message_delta 累计值）：不覆盖，只重算 total
	info := newInterruptedInfo(120)
	usage := &common.Usage{PromptTokens: 80, CompletionTokens: 45}
	ApplyInterruptedUsageFallback(info, usage, 200)
	if usage.PromptTokens != 80 || usage.CompletionTokens != 45 {
		t.Errorf("真实 usage 不应被覆盖，got %+v", usage)
	}
	if usage.TotalTokens != 125 {
		t.Errorf("TotalTokens = %d, want 125", usage.TotalTokens)
	}
}

func TestApplyInterruptedUsageFallback_PartialUsage(t *testing.T) {
	// 中断且只有输出（无输入）：输入补齐，输出保留真实值
	info := newInterruptedInfo(60)
	usage := &common.Usage{CompletionTokens: 30}
	ApplyInterruptedUsageFallback(info, usage, 500)
	if usage.CompletionTokens != 30 {
		t.Errorf("CompletionTokens = %d, want 30（保留真实值）", usage.CompletionTokens)
	}
	if usage.PromptTokens != 60 {
		t.Errorf("PromptTokens = %d, want 60", usage.PromptTokens)
	}
	if usage.TotalTokens != 90 {
		t.Errorf("TotalTokens = %d, want 90", usage.TotalTokens)
	}
}

func TestApplyInterruptedUsageFallback_NilSafe(t *testing.T) {
	ApplyInterruptedUsageFallback(nil, nil, 100)
	ApplyInterruptedUsageFallback(newInterruptedInfo(10), nil, 100)
	ApplyInterruptedUsageFallback(nil, &common.Usage{}, 100)
	usage := &common.Usage{}
	ApplyInterruptedUsageFallback(&common.RelayInfo{}, usage, 100)
	if usage.TotalTokens != 0 {
		t.Errorf("无 StreamStatus 应为 no-op，got %+v", usage)
	}
}
