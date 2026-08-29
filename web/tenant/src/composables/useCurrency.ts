import { computed } from 'vue'
import { usePublicSettings } from '@/composables/usePublicSettings'

/**
 * 全站货币显示（本位币制）：
 * - bil_ 层（钱包/计费/定价/用量/阈值）币种 = 本位币 billing_currency，直显不折算
 * - ord_/pln_ 层（订单/套餐/优惠码折扣）恒 CNY；本位币=USD 时按汇率折算显示，本位币=CNY 直显
 * - 本位币由系统初始化向导选定，前端只读（/settings/public 暴露）
 * - 编辑表单保持存储币种原值输入，折算仅用于只读展示
 */
export type CurrencyCode = 'USD' | 'CNY'

/** 默认 CNY→USD 汇率（与后端 billing.defaultCNYToUSD 一致，配置未加载/无效时兜底） */
const FALLBACK_CNY_TO_USD = 0.14

const { settings } = usePublicSettings()

/** 全站显示货币（= 系统本位币），未加载前保守默认 USD */
export const displayCurrency = computed<CurrencyCode>(() =>
	settings.value.billing_currency === 'CNY' ? 'CNY' : 'USD',
)

/** CNY→USD 汇率（管理后台支付设置配置） */
export const cnyToUsd = computed(() => {
	const r = Number(settings.value.payment_exchange_rate_cny_to_usd)
	return r > 0 ? r : FALLBACK_CNY_TO_USD
})

export interface FormatMoneyOptions {
	/** 数据存储层币种：'USD' | 'CNY'（按字段所属数据层传入，非显示币种） */
	source?: CurrencyCode
	/** 显示精度；默认按显示币种取 USD 6 位 / CNY 2 位 */
	precision?: number
	/** 正负号前缀（交易流水） */
	showSign?: boolean
}

/**
 * 统一金额格式化（跟随本位币）。
 * source='CNY'（订单层）且本位币=USD 时按汇率折算；bil 层数据传 source=displayCurrency 即直显。
 * 内部读取响应式 computed，render 函数中调用可建立依赖，配置变化自动重渲染。
 */
export function formatMoney(value: unknown, opts: FormatMoneyOptions = {}): string {
	const { source = 'USD', precision, showSign = false } = opts
	const display = displayCurrency.value
	let num = Number(value ?? 0)
	// 唯一折算场景：订单层 CNY 金额在本位币 USD 部署下展示
	if (source === 'CNY' && display === 'USD') {
		num = num * cnyToUsd.value
	}
	const symbol = display === 'CNY' ? '¥' : '$'
	const p = precision ?? (display === 'CNY' ? 2 : 6)
	const sign = showSign ? (num > 0 ? '+' : num < 0 ? '-' : '') : ''
	// 先 toFixed 确保精度，再用 Number 去掉末尾的 0
	return `${sign}${symbol}${Number(Math.abs(num).toFixed(p))}`
}

/** bil_ 层（钱包/计费/定价等本位币数据）快捷格式化：直显 + 本位币符号 */
export function formatBilling(value: unknown, precision?: number, showSign = false): string {
	return formatMoney(value, { source: displayCurrency.value, precision, showSign })
}

/** ord_/pln_ 层（订单/套餐等 CNY 数据）快捷格式化：本位币=CNY 直显，USD 折算 */
export function formatOrder(value: unknown, precision?: number, showSign = false): string {
	return formatMoney(value, { source: 'CNY', precision, showSign })
}

/**
 * 充值页专用：本位币 USD 部署下，用户输入的显示货币（USD）金额换算为 CNY 下单金额。
 * 本位币 CNY 时原样返回（充值锚定 CNY，零折算）。
 */
export function displayToCny(value: number): number {
	if (displayCurrency.value === 'CNY') return value
	return Number((value / cnyToUsd.value).toFixed(2))
}

/**
 * 充值页专用：CNY 下单金额换算为显示货币金额（档位折算展示，¥100 × 0.14 = $14）。
 * 本位币 CNY 时原样返回。
 */
export function cnyToDisplay(value: number): number {
	if (displayCurrency.value === 'CNY') return value
	return Number((value * cnyToUsd.value).toFixed(2))
}
