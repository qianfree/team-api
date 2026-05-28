package billing

import (
	"strings"
	"testing"
)

func TestBuildBillingSummary_Basic(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:  "gpt-4o",
		InputTokens:     1000,
		OutputTokens:    500,
		InputPrice:      0.50,
		OutputPrice:     1.50,
		InputCost:       0.0005,
		OutputCost:      0.00075,
		TotalCost:       0.00125,
		PreDeductAmount: 0.01,
		RefundAmount:    0.00875,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "gpt-4o") {
		t.Error("should contain model name")
	}
	if !strings.Contains(text, "输入") {
		t.Error("should contain 输入")
	}
	if !strings.Contains(text, "输出") {
		t.Error("should contain 输出")
	}
	if !strings.Contains(text, "退回") {
		t.Error("should contain 退回")
	}
}

func TestBuildBillingSummary_WithUpstreamModel(t *testing.T) {
	p := &SummaryParams{
		RequestedModel: "gpt-4o",
		UpstreamModel:  "gpt-4o-2024-08-06",
		InputTokens:    100,
		OutputTokens:   50,
		InputPrice:     1.0,
		OutputPrice:    2.0,
		InputCost:      0.0001,
		OutputCost:     0.0001,
		TotalCost:      0.0002,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "gpt-4o → gpt-4o-2024-08-06") {
		t.Error("should show model mapping")
	}
}

func TestBuildBillingSummary_SameUpstreamModel(t *testing.T) {
	p := &SummaryParams{
		RequestedModel: "gpt-4o",
		UpstreamModel:  "gpt-4o",
		InputTokens:    100,
		OutputTokens:   50,
		InputPrice:     1.0,
		OutputPrice:    2.0,
		InputCost:      0.0001,
		OutputCost:     0.0001,
		TotalCost:      0.0002,
	}

	text := BuildBillingSummary(p)
	if strings.Contains(text, "→") {
		t.Error("should not show arrow when upstream == requested")
	}
}

func TestBuildBillingSummary_WithChannelName(t *testing.T) {
	p := &SummaryParams{
		RequestedModel: "gpt-4o",
		ChannelName:    "OpenAI 主力渠道",
		InputTokens:    100,
		OutputTokens:   50,
		InputPrice:     1.0,
		OutputPrice:    2.0,
		InputCost:      0.0001,
		OutputCost:     0.0001,
		TotalCost:      0.0002,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "渠道: OpenAI 主力渠道") {
		t.Error("should show channel name")
	}
}

func TestBuildBillingSummary_PerRequestMode(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:  "dall-e-3",
		BillingMode:     "per_request",
		TotalCost:       0.04,
		PreDeductAmount: 0.04,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "per_request") {
		t.Error("should show billing mode")
	}
}

func TestBuildBillingSummary_Discount(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:  "gpt-4o",
		InputTokens:     1000,
		InputPrice:      1.0,
		InputCost:       0.001,
		TotalCost:       0.001,
		DiscountRatio:   0.85,
		ActualCost:      0.00085,
		PreDeductAmount: 0.001,
		RefundAmount:    0.00015,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "折扣") {
		t.Error("should show discount info")
	}
}

func TestBuildBillingSummary_NoDiff(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:  "gpt-4o",
		InputTokens:     100,
		InputPrice:      1.0,
		InputCost:       0.0001,
		TotalCost:       0.0001,
		PreDeductAmount: 0.0001,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "无差额") {
		t.Error("should show 无差额 when no refund/supplement")
	}
}

func TestBuildBillingSummary_Supplement(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:   "gpt-4o",
		InputTokens:      100,
		InputPrice:       1.0,
		InputCost:        0.001,
		TotalCost:        0.001,
		PreDeductAmount:  0.0005,
		SupplementAmount: 0.0005,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "补扣") {
		t.Error("should show 补扣")
	}
}

func TestBuildBillingSummary_CacheRead(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:  "claude-3-5-sonnet",
		InputTokens:     1000,
		CacheReadTokens: 500,
		CacheReadPrice:  0.10,
		CacheReadCost:   0.00005,
		TotalCost:       0.00005,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "缓存读取") {
		t.Error("should show cache read line")
	}
}

func TestBuildBillingSummary_CacheCreation(t *testing.T) {
	p := &SummaryParams{
		RequestedModel:      "claude-3-5-sonnet",
		InputTokens:         1000,
		CacheCreationTokens: 300,
		TotalCost:           0.0001,
	}

	text := BuildBillingSummary(p)
	if !strings.Contains(text, "缓存创建") {
		t.Error("should show cache creation line")
	}
}

func TestBuildBillingSummary_NoPreDeduct(t *testing.T) {
	p := &SummaryParams{
		RequestedModel: "gpt-4o",
		InputTokens:    100,
		InputPrice:     1.0,
		InputCost:      0.0001,
		TotalCost:      0.0001,
	}

	text := BuildBillingSummary(p)
	if strings.Contains(text, "预扣") {
		t.Error("should not show 预扣 when PreDeductAmount = 0")
	}
}

func TestFormatInt(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1000, "1000"},
		{999999, "999999"},
	}
	for _, tt := range tests {
		got := formatInt(tt.input)
		if got != tt.expected {
			t.Errorf("formatInt(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0.000000"},
		{0.001, "0.001000"},
		{1.234567, "1.234567"},
	}
	for _, tt := range tests {
		got := formatCost(tt.input)
		if got != tt.expected {
			t.Errorf("formatCost(%f) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
