package billing

import (
	"testing"
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
	// 两个默认汇率不应精确互为倒数（市场汇率有买卖价差），但数量级应一致
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

func TestCurrencyConstants_ReasonableRange(t *testing.T) {
	// CNY→USD 应在 0.1~0.2 范围内（合理汇率区间）
	if defaultCNYToUSD < 0.05 || defaultCNYToUSD > 0.3 {
		t.Errorf("defaultCNYToUSD %f outside reasonable range [0.05, 0.3]", defaultCNYToUSD)
	}
	// USD→CNY 应在 5~15 范围内
	if defaultUSDToCNY < 5 || defaultUSDToCNY > 15 {
		t.Errorf("defaultUSDToCNY %f outside reasonable range [5, 15]", defaultUSDToCNY)
	}
}
