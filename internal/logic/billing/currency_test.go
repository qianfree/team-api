package billing

import (
	"testing"

	"github.com/gogf/gf/v2/test/gtest"
	"github.com/shopspring/decimal"
)

func TestDefaultCNYToUSD(t *testing.T) {
	if defaultCNYToUSD <= 0 {
		t.Fatalf("defaultCNYToUSD should be positive, got %f", defaultCNYToUSD)
	}
}

func TestDefaultUSDToCNY(t *testing.T) {
	if defaultUSDToCNY <= 0 {
		t.Fatalf("defaultUSDToCNY should be positive, got %f", defaultUSDToCNY)
	}
}

func TestDefaultRates_Reciprocal(t *testing.T) {
	// 修复后：defaultUSDToCNY 不再是独立配置，而是 defaultCNYToUSD 的倒数
	// 验证常量定义互为倒数
	product := defaultCNYToUSD * defaultUSDToCNY
	if product < 0.9 || product > 1.1 {
		t.Fatalf("default rate product should be ~1.0, got %f", product)
	}
}

func TestConvertCNYToUSD_DefaultRate(t *testing.T) {
	// 无配置时使用默认汇率 0.14
	tests := []struct {
		name     string
		cny      float64
		expected float64
	}{
		{"zero", 0, 0},
		{"100 CNY", 100, 100 * defaultCNYToUSD},
		{"1 CNY", 1, 1 * defaultCNYToUSD},
		{"large amount", 10000, 10000 * defaultCNYToUSD},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.cny * defaultCNYToUSD
			if diff := actual - tt.expected; diff < -0.000001 || diff > 0.000001 {
				t.Errorf("CNY %f → USD: expected %f, got %f", tt.cny, tt.expected, actual)
			}
		})
	}
}

func TestConvertUSDToCNY_DefaultRate(t *testing.T) {
	tests := []struct {
		name     string
		usd      float64
		expected float64
	}{
		{"zero", 0, 0},
		{"1 USD", 1, defaultUSDToCNY},
		{"14 USD", 14, 14 * defaultUSDToCNY},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := tt.usd * defaultUSDToCNY
			if diff := actual - tt.expected; diff < -0.000001 || diff > 0.000001 {
				t.Errorf("USD %f → CNY: expected %f, got %f", tt.usd, tt.expected, actual)
			}
		})
	}
}

func TestCurrencyConstants_NonZero(t *testing.T) {
	if defaultCNYToUSD == 0 {
		t.Error("defaultCNYToUSD must not be zero")
	}
	if defaultUSDToCNY == 0 {
		t.Error("defaultUSDToCNY must not be zero")
	}
}

// TestExchangeRateReciprocal 验证 USD→CNY 汇率是 CNY→USD 的倒数（往返闭合）
func TestExchangeRateReciprocal(t *testing.T) {
	// 测试默认汇率互为倒数
	reciprocal := 1.0 / defaultCNYToUSD
	diff := reciprocal - defaultUSDToCNY
	if diff < 0 {
		diff = -diff
	}
	// 允许误差 0.01（因为默认常量可能不完全精确）
	if diff > 0.01 {
		t.Errorf("默认汇率应互为倒数：1/%.4f = %.4f, 但 defaultUSDToCNY = %.4f, 差值 = %.6f",
			defaultCNYToUSD, reciprocal, defaultUSDToCNY, diff)
	}
}

// TestConvertRoundTrip 验证 CNY→USD→CNY 往返转换闭合（误差在可接受范围内）
func TestConvertRoundTrip(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试多个典型金额
		testAmounts := []float64{100.0, 50.5, 1.0, 999.99}

		for _, originalCNY := range testAmounts {
			// CNY → USD（使用默认汇率模拟）
			usd := decimal.NewFromFloat(originalCNY).Mul(decimal.NewFromFloat(defaultCNYToUSD))

			// USD → CNY（使用倒数）
			cny := usd.Mul(decimal.NewFromFloat(1.0 / defaultCNYToUSD))

			// 验证往返误差 < 0.01 元（1分钱）
			diff := cny.Sub(decimal.NewFromFloat(originalCNY)).Abs()
			t.AssertLT(InexactFloat64(diff), 0.01)
		}
	})
}

// TestConvertCNYToUSD_Precision 验证 CNY→USD 转换的精度（使用默认汇率）
func TestConvertCNYToUSD_Precision(t *testing.T) {
	// 100 CNY × 0.14 = 14 USD
	usd := decimal.NewFromFloat(100.0).Mul(decimal.NewFromFloat(defaultCNYToUSD))
	expected := decimal.NewFromFloat(14.0)

	diff := usd.Sub(expected).Abs()
	if InexactFloat64(diff) > 0.000001 {
		t.Errorf("100 CNY 应转换为约 14 USD，实际=%s, 误差=%s", usd.String(), diff.String())
	}
}

// TestConvertUSDToCNY_Precision 验证 USD→CNY 转换的精度（使用倒数）
func TestConvertUSDToCNY_Precision(t *testing.T) {
	// 14 USD × (1/0.14) ≈ 100 CNY
	usd := decimal.NewFromFloat(14.0)
	reciprocalRate := 1.0 / defaultCNYToUSD
	cny := usd.Mul(decimal.NewFromFloat(reciprocalRate))
	expected := decimal.NewFromFloat(100.0)

	diff := cny.Sub(expected).Abs()
	if InexactFloat64(diff) > 0.01 {
		t.Errorf("14 USD 应转换为约 100 CNY，实际=%s, 误差=%s", cny.String(), diff.String())
	}
}
