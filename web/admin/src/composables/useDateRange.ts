import type { ShortcutType } from '@arco-design/web-vue'

// 日期辅助（native，避免引入 dayjs 依赖）
function pad2(n: number): string {
	return String(n).padStart(2, '0')
}
function toDateTimeStr(d: Date): string {
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} ${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
}
// 默认查询当天：起始时间为当天 0 点，截止时间留空（后端按「到现在」实时处理）。
// 日期选择器里的「现在」仅用于展示；查询时截止仍为默认「现在」则不传 end_date，避免固定截止时间漏掉后续新记录。
// 该值在模块加载时固定一次，页面间共享，保证「截止等于默认现在则不下发 end_date」的判据一致。
// 指定偏移天数的当天 0 点 / 23:59:59（offsetDays=0 表示今天）
function dateStartStr(offsetDays: number): string {
	const d = new Date()
	d.setDate(d.getDate() - offsetDays)
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 00:00:00`
}
function dateEndStr(offsetDays: number): string {
	const d = new Date()
	d.setDate(d.getDate() - offsetDays)
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 23:59:59`
}
// 指定偏移天数的日期（YYYY-MM-DD，不含时间）
function dateOnlyStr(offsetDays: number): string {
	const d = new Date()
	d.setDate(d.getDate() - offsetDays)
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

// 日期时间选择器底部快捷选项：今天 / 昨天 / 近三天 / 近一周。
// 截止「现在」的选项复用 defaultEnd，保持「截止不传 → 后端按到现在实时查询」的语义。
export function useDateRange() {
	// 默认展示截止时间预留 1 小时；查询仍识别此默认值并省略 end_date，
	// 因而后端会实时查询到当前时刻，用户停留页面后刷新也不会漏掉新记录。
	const defaultEndDate = new Date()
	defaultEndDate.setHours(defaultEndDate.getHours() + 1)
	const defaultEnd = toDateTimeStr(defaultEndDate)
	const defaultTodayRange = (): string[] => {
		const d = new Date()
		return [`${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 00:00:00`, defaultEnd]
	}
	const quickDateRanges: ShortcutType[] = [
		{ label: '今天', value: defaultTodayRange },
		{ label: '昨天', value: () => [dateStartStr(1), dateEndStr(1)] },
		{ label: '近三天', value: () => [dateStartStr(2), defaultEnd] },
		{ label: '近一周', value: () => [dateStartStr(6), defaultEnd] },
	]
	return { defaultEnd, defaultTodayRange, quickDateRanges }
}

// 按天粒度（YYYY-MM-DD）的快捷区间，供订单、交易流水等按整日闭区间过滤的页面使用。
// 这类页面的后端约定传日期字符串（自行拼接 00:00:00 / 23:59:59），
// 不能复用上面的 datetime 快捷项——带时分的值会被后端再拼一次时间导致非法日期。
export function useDayShortcuts(): ShortcutType[] {
	return [
		{ label: '今天', value: () => [dateOnlyStr(0), dateOnlyStr(0)] },
		{ label: '昨天', value: () => [dateOnlyStr(1), dateOnlyStr(1)] },
		{ label: '近七天', value: () => [dateOnlyStr(6), dateOnlyStr(0)] },
		{ label: '近三十天', value: () => [dateOnlyStr(29), dateOnlyStr(0)] },
	]
}
