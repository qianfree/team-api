package billing

import (
	"testing"

	"github.com/shopspring/decimal"
)

// TestRoundMoney 验证四舍五入到 10 位小数（对齐 NUMERIC(20,10)）。
func TestRoundMoney(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"0.30000000000000004", 0.3},
		{"0.12345678905", 0.1234567891}, // 第 11 位进位
		{"0.12345678904", 0.123456789},  // 第 11 位舍去
		{"1", 1},
	}
	for _, c := range cases {
		d, _ := decimal.NewFromString(c.in)
		if got := roundMoney(d); got != c.want {
			t.Fatalf("roundMoney(%s) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestComputeCostDecimalPrecision 验证 token 成本用 decimal 精确计算：
// 100000 token × $3/1M = $0.3。float64 链式运算会得到 0.30000000000000004，
// decimal 精确运算 + 四舍五入到 10 位得到精确的 0.3。
func TestComputeCostDecimalPrecision(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		InputPrice:       3.0,
		OutputPrice:      0,
		TenantMultiplier: 1.0,
		Currency:         "USD",
	}
	bd := computeCost(pricing, 100000, 0, nil)
	if bd.TotalCost != 0.3 {
		t.Fatalf("TotalCost = %v (%.20f), want exactly 0.3", bd.TotalCost, bd.TotalCost)
	}
	if bd.InputCost != 0.3 {
		t.Fatalf("InputCost = %v, want 0.3", bd.InputCost)
	}
}

// TestMicroRoundTrip 验证 USD ↔ 整数微单位换算的正确性与精确取整。
func TestMicroRoundTrip(t *testing.T) {
	cases := []struct {
		usd       float64
		wantMicro int64
	}{
		{0, 0},
		{1, 1_000_000},
		{0.1, 100_000},
		{0.015, 15_000},
		{0.000001, 1},
		{0.0000004, 0}, // 四舍五入到 0
		{0.0000006, 1}, // 四舍五入到 1 micro
		{12345.678901, 12_345_678_901},
	}
	for _, c := range cases {
		if got := toMicro(c.usd); got != c.wantMicro {
			t.Fatalf("toMicro(%v) = %d, want %d", c.usd, got, c.wantMicro)
		}
	}
	// fromMicro 反向
	if got := fromMicro(15_000); got != 0.015 {
		t.Fatalf("fromMicro(15000) = %v, want 0.015", got)
	}
	if got := fromMicro(1_000_000); got != 1.0 {
		t.Fatalf("fromMicro(1000000) = %v, want 1.0", got)
	}
}

// (100000/1M × $3) × 1.1 = 0.33。
func TestComputeCostWithMultiplier(t *testing.T) {
	pricing := &PricingResult{
		BillingMode:      "token",
		InputPrice:       3.0,
		TenantMultiplier: 1.1,
		Currency:         "USD",
	}
	bd := computeCost(pricing, 100000, 0, nil)
	if bd.TotalCost != 0.33 {
		t.Fatalf("TotalCost = %v (%.20f), want exactly 0.33", bd.TotalCost, bd.TotalCost)
	}
}
