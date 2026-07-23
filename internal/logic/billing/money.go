package billing

import "github.com/shopspring/decimal"

// 金额精度工具（decimal 原生版，任务 #6）。
//
// 背景：系统所有资金列已从 float64 迁移为 decimal.Decimal（entity 层 + 计算层全链路）。
// 本文件提供 decimal 原生运算工具、精度常量、边界转换（decimal ↔ float64 / int64 micro）。
//
// 核心原则：
// - entity/计算层全程 decimal，消除浮点累积误差
// - Res 响应出口调 .InexactFloat64() 转回 float64（保持前端 JSON number 契约）
// - Redis 钱包用整数 micro-USD（int64 HINCRBY），toMicro/fromMicro 负责转换

const (
	// MoneyScale 金额存储精度：与 DB 列 NUMERIC(20,10) 对齐。
	MoneyScale = 10

	// UsdDisplayScale 钱包/展示精度：小数点后 6 位（见 CLAUDE.md 资金显示精度）。
	UsdDisplayScale = 6

	// MicroScale 整数微单位换算比例：1 USD = 1_000_000 micro（6 位小数，钱包/展示精度）。
	// Redis 钱包金额以整数 micro 存储，用整数加减（HINCRBY）替代 HINCRBYFLOAT，
	// 彻底消除 IEEE double 累计漂移。int64 上限 ~9.2e18，按 micro 计可表示约 $9.2 万亿/租户，余量充足。
	MicroScale = 1_000_000
)

// 预定义常量（减少重复 NewFromInt 调用）
var (
	Million      = decimal.NewFromInt(MicroScale)        // 1_000_000，token 单价换算
	MoneyEpsilon = decimal.NewFromFloat(0.000001)        // 1 微美元容差（浮点比较遗留兼容）
	Zero         = decimal.Zero                          // 常用零值
	One          = decimal.NewFromInt(1)                 // 常用 1
)

// RoundMoney 将 decimal 金额四舍五入到 10 位小数（与 NUMERIC(20,10) 对齐）。
// 用于把 decimal 精确计算的结果收敛到存储精度，作为落库前的统一出口。
func RoundMoney(d decimal.Decimal) decimal.Decimal {
	return d.Round(MoneyScale)
}

// RoundUSD 将 decimal 金额四舍五入到 6 位小数（钱包 USD 精度）。
// 用于展示/API 响应的精度收敛，与 CLAUDE.md 资金显示精度对齐。
func RoundUSD(d decimal.Decimal) decimal.Decimal {
	return d.Round(UsdDisplayScale)
}

// CeilUSD 将 decimal 金额向上取整到 6 位小数（钱包 USD 精度）。
// 用于 CNY→USD 充值入账：宁可多给用户零头也不少给，且与展示精度一致。
func CeilUSD(d decimal.Decimal) decimal.Decimal {
	return d.RoundCeil(UsdDisplayScale)
}

// ToMicro 将 USD 金额（decimal）四舍五入换算为整数微单位（int64）。
// 用于 Redis 钱包 HINCRBY 操作（整数加减，无浮点漂移）。
func ToMicro(usd decimal.Decimal) int64 {
	return usd.Mul(Million).Round(0).IntPart()
}

// FromMicro 将整数微单位（int64）换算回 USD（decimal）。
// 用于从 Redis 钱包读取余额后转回 decimal 参与计算。
func FromMicro(micro int64) decimal.Decimal {
	return decimal.New(micro, -6) // 等价于 micro / 1_000_000，但更高效
}

// InexactFloat64 将 decimal 转为 float64（有损转换，供边界使用）。
// 用途：
// - Res 响应出口（保持前端 JSON number 契约，任务 #15）
// - relay 缓存结构体（内存缓存用 float64，避免 decimal 序列化开销）
// - 遗留外部接口（如支付回调需要 float64 参数）
// ⚠️ 禁止在计算链路中使用，只允许在输出边界调用。
func InexactFloat64(d decimal.Decimal) float64 {
	f, _ := d.Float64() // 忽略 inexact 标志，调用方已知有损
	return f
}

// NewFromFloat 从 float64 创建 decimal（输入边界，供 API Req 转换用）。
// decimal.NewFromFloat 取能往返的最短十进制表示，0.1 → "0.1" 而非 0.1000000000000000055…，
// 不会把浮点噪声带进精确运算。
func NewFromFloat(f float64) decimal.Decimal {
	return decimal.NewFromFloat(f)
}

// MultiplyMoney 金额精确乘法（a × b）。
// 用于替代裸乘法，避免链式运算累积误差（如 price × months、amount × discount）。
// 结果自动 RoundMoney 收敛到 10 位小数。
func MultiplyMoney(a, b decimal.Decimal) decimal.Decimal {
	return RoundMoney(a.Mul(b))
}

// DivideMoney 金额精确除法（a ÷ b）。
// 用于替代裸除法（如 totalCost / requests）。
// 结果自动 RoundMoney 收敛到 10 位小数。
func DivideMoney(a, b decimal.Decimal) decimal.Decimal {
	return RoundMoney(a.Div(b))
}

// AddMoney 金额精确加法（a + b）。
// 用于循环累加避免误差放大（虽然 decimal 加法无误差，但统一收敛规范）。
func AddMoney(a, b decimal.Decimal) decimal.Decimal {
	return RoundMoney(a.Add(b))
}

// SubtractMoney 金额精确减法（a - b）。
// 用于差额计算（如 available = balance - frozen）。
func SubtractMoney(a, b decimal.Decimal) decimal.Decimal {
	return RoundMoney(a.Sub(b))
}

// IsPositive 判断金额是否为正数（> 0）。
func IsPositive(d decimal.Decimal) bool {
	return d.GreaterThan(Zero)
}

// IsZeroOrNegative 判断金额是否为零或负数（<= 0）。
func IsZeroOrNegative(d decimal.Decimal) bool {
	return d.LessThanOrEqual(Zero)
}

// Max 返回两个金额中的较大值。
func Max(a, b decimal.Decimal) decimal.Decimal {
	if a.GreaterThan(b) {
		return a
	}
	return b
}

// Min 返回两个金额中的较小值。
func Min(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}
