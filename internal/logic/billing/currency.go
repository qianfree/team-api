package billing

import (
	"context"
	"math"

	lcommon "github.com/qianfree/team-api/internal/logic/common"
)

const defaultCNYToUSD = 0.14
const defaultUSDToCNY = 7.25

// usdPrecision 钱包余额精度：小数点后 6 位。
const usdPrecision = 1e6

// ConvertCNYToUSD 将人民币金额转换为美元，向上取整到小数点后 6 位。
func ConvertCNYToUSD(ctx context.Context, cnyAmount float64) float64 {
	rate := GetExchangeRateCNYToUSD(ctx)
	return math.Ceil(cnyAmount*rate*usdPrecision) / usdPrecision
}

// ConvertUSDToCNY 将美元金额转换为人民币
func ConvertUSDToCNY(ctx context.Context, usdAmount float64) float64 {
	rate := GetExchangeRateUSDToCNY(ctx)
	return usdAmount * rate
}

// GetExchangeRateCNYToUSD 获取 CNY→USD 兑换比例
func GetExchangeRateCNYToUSD(ctx context.Context) float64 {
	cfg := lcommon.Config()
	rate := cfg.GetFloat(ctx, "payment_exchange_rate_cny_to_usd")
	if rate <= 0 {
		return defaultCNYToUSD
	}
	return rate
}

// GetExchangeRateUSDToCNY 获取 USD→CNY 兑换比例
func GetExchangeRateUSDToCNY(ctx context.Context) float64 {
	cfg := lcommon.Config()
	rate := cfg.GetFloat(ctx, "payment_exchange_rate_usd_to_cny")
	if rate <= 0 {
		return defaultUSDToCNY
	}
	return rate
}
