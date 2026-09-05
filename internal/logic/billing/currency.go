package billing

import (
	"context"

	"github.com/qianfree/team-api/internal/consts"
	lcommon "github.com/qianfree/team-api/internal/logic/common"
	"github.com/shopspring/decimal"
)

const defaultCNYToUSD = 0.14
const defaultUSDToCNY = 7.142857142857143 // 1 / 0.14，确保互为倒数

// Currency 返回系统本位币（bil_ 记账层的记账币种与全站显示货币）。
// 本位币在系统初始化向导选定后不可更改；存量部署与未初始化系统默认 USD。
// 配置基础设施不可用（如单测环境无 DB 驱动，g.DB() 直接 panic）时回退 USD，
// 与存量部署行为一致，保证纯函数单测可运行。
func Currency(ctx context.Context) (cur string) {
	defer func() {
		if r := recover(); r != nil {
			cur = consts.BillingCurrencyUSD
		}
	}()
	c := lcommon.Config().GetString(ctx, consts.OptionKeyBillingCurrency)
	if c == consts.BillingCurrencyCNY {
		return consts.BillingCurrencyCNY
	}
	return consts.BillingCurrencyUSD
}

// IsCNY 当前本位币是否为人民币（此时充值履约直接入账、无换汇环节）
func IsCNY(ctx context.Context) bool {
	return Currency(ctx) == consts.BillingCurrencyCNY
}

// CurrencySymbol 返回本位币的货币符号，用于服务端生成的展示文本
// （计费摘要、CSV/Excel 导出等前端拿不到结构化金额、无法走 useCurrency 的场景）。
// 仅适用于 bil_ 记账层金额；ord_/pln_ 层金额恒为 CNY，直接写 ¥，不要用本函数。
func CurrencySymbol(ctx context.Context) string {
	if Currency(ctx) == consts.BillingCurrencyCNY {
		return "¥"
	}
	return "$"
}

// ConvertCNYToUSD 将人民币金额转换为美元，向上取整到小数点后 6 位（decimal 原生版）。
// 用 decimal 精确乘法替代 float64 链式运算，避免 cnyAmount×rate 的浮点误差。
func ConvertCNYToUSD(ctx context.Context, cnyAmount float64) decimal.Decimal {
	rate := GetExchangeRateCNYToUSD(ctx)
	return CeilUSD(NewFromFloat(cnyAmount).Mul(NewFromFloat(rate)))
}

// ConvertUSDToCNY 将美元金额转换为人民币（decimal 原生版）。
// decimal 精确乘法 + 四舍五入到存储精度（10 位），消除 float64 累计误差。
func ConvertUSDToCNY(ctx context.Context, usdAmount decimal.Decimal) decimal.Decimal {
	rate := GetExchangeRateUSDToCNY(ctx)
	return RoundMoney(usdAmount.Mul(NewFromFloat(rate)))
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

// GetExchangeRateUSDToCNY 获取 USD→CNY 兑换比例（取 CNY→USD 的倒数，确保往返闭合）
func GetExchangeRateUSDToCNY(ctx context.Context) float64 {
	rate := GetExchangeRateCNYToUSD(ctx)
	if rate <= 0 {
		return defaultUSDToCNY
	}
	return 1.0 / rate
}
