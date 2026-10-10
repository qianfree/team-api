package tenant

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/qianfree/team-api/internal/logic/payment"
)

// TestComputeTierDiscount 档位折扣基数计算口径：仅整数金额精确命中档位时生效。
// 该函数由 RechargeCreate 下单与 ValidatePromoCode 预检共用，两侧折扣基数必须一致。
func TestComputeTierDiscount(t *testing.T) {
	settings := &payment.GlobalPaymentSettings{
		AmountDiscount: map[int]float64{100: 0.9, 500: 0.85, 200: 0},
	}

	cases := []struct {
		name     string
		settings *payment.GlobalPaymentSettings
		amount   string // decimal 字符串，避免浮点字面量误差
		wantBase string
		wantTier string
	}{
		{"整数命中 100 档", settings, "100", "90", "10"},
		{"整数命中 500 档", settings, "500", "425", "75"},
		{"非整数不命中（100.99 不得享 100 档）", settings, "100.99", "100.99", "0"},
		{"整数但无档位配置", settings, "50", "50", "0"},
		{"档位折扣非正值忽略", settings, "200", "200", "0"},
		{"无支付配置", nil, "100", "100", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, tier := computeTierDiscount(tc.settings, decimal.RequireFromString(tc.amount))
			assertDecimalEq(t, "finalBase", base, tc.wantBase)
			assertDecimalEq(t, "tierDiscount", tier, tc.wantTier)
		})
	}
}

func assertDecimalEq(t *testing.T, field string, got decimal.Decimal, want string) {
	t.Helper()
	wantD := decimal.RequireFromString(want)
	if !got.Equal(wantD) {
		t.Fatalf("%s = %s, want %s", field, got.String(), want)
	}
}
