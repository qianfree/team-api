import { h } from 'vue'
import { NTag } from 'naive-ui'
import { formatMoney as formatMoneyWithCurrency } from '@/composables/useCurrency'

/**
 * NDataTable 迁移公共渲染工具。
 * 汇总各页重复的 badge/金额/日期/Token 格式化逻辑，统一到 Naive 渲染。
 */

/** 现有 badge class → NTag type 映射 */
export const BADGE_TYPE_MAP: Record<string, 'default' | 'success' | 'info' | 'warning' | 'error' | 'primary'> = {
	'badge-primary': 'info',
	'badge-success': 'success',
	'badge-warning': 'warning',
	'badge-danger': 'error',
	'badge-gray': 'default',
	'badge-purple': 'primary',
}

/**
 * 渲染状态徽章：根据 value 从 labelMap/colorMap 取文案与颜色，输出 NTag。
 * @param value   单元格值
 * @param labelMap 值 → 中文文案（可选）
 * @param colorMap 值 → badge class（可选，用于 NTag type）
 */
export function renderBadge(
	value: unknown,
	labelMap?: Record<string, string>,
	colorMap?: Record<string, string>,
) {
	const label = labelMap?.[String(value)] ?? (value as string) ?? ''
	const cls = colorMap?.[String(value)] ?? 'badge-gray'
	const type = BADGE_TYPE_MAP[cls] ?? 'default'
	return h(NTag, { type, size: 'small' }, { default: () => label })
}

export interface FormatMoneyOptions {
	/** 数据存储层币种（按字段所属层传入：bil 层 USD/CNY 由本位币定，订单层 CNY），非显示币种 */
	currency?: 'USD' | 'CNY'
	precision?: number
	showSign?: boolean
}

/**
 * 统一金额格式化（委托 useCurrency，跟随系统本位币显示）。
 * currency 参数语义 = 数据存储层币种：bil 层数据传本位币对应值（USD 部署默认 USD）、
 * 订单/充值层传 'CNY'（本位币 USD 时自动按汇率折算显示）。
 * 精度默认按显示币种取 USD 6 位 / CNY 2 位；showSign 用 +/− 前缀（交易流水）。
 * 自动去掉末尾的 0（如 $1.500000 → $1.5）。
 */
export function formatMoney(value: unknown, opts: FormatMoneyOptions = {}): string {
	const { currency = 'USD', precision, showSign = false } = opts
	return formatMoneyWithCurrency(value, { source: currency, precision, showSign })
}

/** 统一日期格式化：ISO T 转空格、截断到分钟；空值显示 -- */
export function formatDate(value: unknown): string {
	if (value == null || value === '') return '--'
	const s = String(value).replace('T', ' ').substring(0, 16)
	return s
}

/** Token 数量缩写：K / M */
export function formatTokens(value: unknown): string {
	const num = Number(value ?? 0)
	if (num >= 1_000_000) return `${(num / 1_000_000).toFixed(1)}M`
	if (num >= 1_000) return `${(num / 1_000).toFixed(1)}K`
	return String(num)
}

/** 延迟毫秒格式化 */
export function formatMs(value: unknown): string {
	if (value == null || value === '') return '-'
	return `${Math.round(Number(value))}ms`
}

/**
 * 计算 NDataTable 的 scroll-x：取所有列 width 之和。
 * 搭配每列都有 width 的用法，桌面端表格按 width 比例撑满容器、
 * 移动端（容器 < 列宽和）触发横向滚动。
 */
export function tableScrollX(columns: { width?: number | string }[]): number {
	return columns.reduce((sum, col) => sum + (Number(col.width) || 0), 0)
}