import { computed } from 'vue'
import { usePublicSettings } from '@/composables/usePublicSettings'

/**
 * 全站货币显示（本位币制），与租户端 useCurrency 镜像：
 * - bil_ 层（钱包/计费/定价/用量/阈值）币种 = 本位币 billing_currency，直显不折算
 * - ord_/pln_ 层（订单/套餐/优惠码折扣）恒 CNY；本位币=USD 时按汇率折算显示，本位币=CNY 直显
 * - 本位币由系统初始化向导选定，管理后台只读（保存支付设置后调用 refresh 立即生效）
 * - 编辑表单保持存储币种原值输入，折算仅用于只读展示
 */
export type CurrencyCode = 'USD' | 'CNY'

/** 默认 CNY→USD 汇率（与后端 billing.defaultCNYToUSD 一致，配置未加载/无效时兜底） */
const FALLBACK_CNY_TO_USD = 0.14

const { settings, fetchSettings } = usePublicSettings()

/** 全站显示货币（= 系统本位币），未加载前保守默认 USD */
export const displayCurrency = computed<CurrencyCode>(() =>
	settings.value.billing_currency === 'CNY' ? 'CNY' : 'USD',
)

/** CNY→USD 汇率（支付设置配置，系统设置保存后 refresh 生效） */
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

/** 强制刷新公共配置（系统设置保存后调用，让已打开页面立即按新汇率/本位币重渲染） */
export function refreshCurrencySettings(): Promise<void> {
	return fetchSettings(true)
}
