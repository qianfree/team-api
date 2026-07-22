package billing

import "github.com/shopspring/decimal"

// 金额精度工具（A8 Phase 1）。
//
// 背景：系统所有资金列均为 PostgreSQL NUMERIC(20,10)，DB 端加减为精确定点运算；
// 但 Go 层历史上用 float64 承载金额，链式乘除（如 tokens/1e6 × 单价 × 租户倍率、CNY×汇率）
// 会累积二进制浮点误差，违反「金额不用 FLOAT」规范。本文件提供 decimal 精确计算 + 统一
// 四舍五入的边界工具：在纯计算处用 decimal 运算，最终在边界收敛到固定精度再落库/返回，
// 消除 float64 累计漂移。Phase 1 仅覆盖换汇与成本汇总，不改动生成的 entity 与 Redis Lua。

const (
	// moneyScale 金额存储精度：与 DB 列 NUMERIC(20,10) 对齐。
	moneyScale = 10
	// usdDisplayScale 钱包/展示精度：小数点后 6 位（见 CLAUDE.md 资金显示精度）。
	usdDisplayScale = 6
)

// dec 把 float64 安全转成 decimal。decimal.NewFromFloat 取能往返的最短十进制表示，
// 因此 0.1 → "0.1" 而非 0.1000000000000000055…，不会把浮点噪声带进精确运算。
func dec(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

// roundMoney 将 decimal 金额四舍五入到 10 位小数（与 NUMERIC(20,10) 对齐）后返回 float64。
// 用于把 decimal 精确计算的结果收敛到存储精度，作为 Go 层金额的统一出口。
func roundMoney(d decimal.Decimal) float64 {
	f, _ := d.Round(moneyScale).Float64()
	return f
}

// ceilUSD 将 decimal 金额向上取整到 6 位小数（钱包 USD 精度）后返回 float64。
// 用于 CNY→USD 充值入账：宁可多给用户零头也不少给，且与展示精度一致。
func ceilUSD(d decimal.Decimal) float64 {
	f, _ := d.RoundCeil(usdDisplayScale).Float64()
	return f
}

// microScale 整数微单位换算比例：1 USD = 1_000_000 micro（6 位小数，钱包/展示精度）。
// Redis 钱包金额以整数 micro 存储（Phase 3），用整数加减（HINCRBY）替代 HINCRBYFLOAT，
// 彻底消除 IEEE double 累计漂移。int64 上限 ~9.2e18，按 micro 计可表示约 $9.2 万亿/租户，余量充足。
const microScale = 1_000_000

// toMicro 将 USD 金额（float64）四舍五入换算为整数微单位（用 decimal 精确取整）。
func toMicro(usd float64) int64 {
	return dec(usd).Mul(decimal.NewFromInt(microScale)).Round(0).IntPart()
}

// fromMicro 将整数微单位换算回 USD（float64）。
func fromMicro(micro int64) float64 {
	f, _ := decimal.New(micro, -6).Float64()
	return f
}
