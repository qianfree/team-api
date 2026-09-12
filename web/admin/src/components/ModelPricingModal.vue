<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCloudDownload, IconRefresh, IconDelete } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { displayCurrency, currencySymbol, formatBilling, cnyToUsd } from '@/composables/useCurrency'

// 本位币符号：定价输入控件后缀跟随本位币，输入值仍为 bil 层存储原值不折算

const props = defineProps<{
	visible: boolean
	modelId: number | null
	modelIdStr: string
}>()

const emit = defineEmits<{
	'update:visible': [value: boolean]
	saved: []
}>()

// ============================================================
// Pricing Editor State
// ============================================================
const editorLoading = ref(false)
const editorSaving = ref(false)
const editorBillingMode = ref('token')
const editorItems = reactive<any[]>([])

// 展示字段（挂锚点行：内部备注 + 对外营销文案）
const editorPriceNote = ref('')
const editorDiscountLabel = ref('')
const editorPriceChangeNote = ref('')

const billingModeOptions = [
	{ label: '按量计费 (Token)', value: 'token' },
	{ label: '按次计费', value: 'per_request' },
	{ label: '阶梯计费', value: 'tiered' },
	{ label: '按秒计费 (视频)', value: 'per_second' },
]

function createEmptyItem(mode: string) {
	return {
		billing_mode: mode,
		min_tokens: 0,
		max_tokens: null,
		input_price: 0,
		output_price: 0,
		per_request_price: null,
		cache_read_price: 0,
		cache_creation_price: 0,
		per_second_prices: null as Record<string, number> | null,
	}
}

// ============================================================
// Per-Second Mode（按秒计费：分辨率规格 × 每秒单价矩阵）
// ============================================================
interface PerSecondRow {
	spec: string // 分辨率/规格键，如 480p / 720p / 1080p / *（兜底）
	price: number // 每秒单价（本位币）
}
const perSecondRows = reactive<PerSecondRow[]>([])

// 常用规格快捷键（点击追加行）
const perSecondSpecPresets = ['480p', '720p', '1080p', '*']

function addPerSecondRow(spec = '') {
	perSecondRows.push({ spec, price: 0 })
}

function removePerSecondRow(index: number) {
	perSecondRows.splice(index, 1)
}

// 回显：后端 map → 行编辑器
function loadPerSecondPrices(prices: Record<string, number> | null | undefined) {
	perSecondRows.length = 0
	if (!prices) return
	for (const [spec, price] of Object.entries(prices)) {
		perSecondRows.push({ spec, price: Number(price) || 0 })
	}
}

// 行 → 提交 map；返回 null 表示校验失败
function buildPerSecondPrices(): Record<string, number> | null {
	const map: Record<string, number> = {}
	let hasPositive = false
	for (const row of perSecondRows) {
		const spec = row.spec.trim()
		if (!spec) continue
		if (map[spec] !== undefined) {
			Message.warning(`按秒计费规格「${spec}」重复`)
			return null
		}
		if (row.price < 0) {
			Message.warning(`按秒计费规格「${spec}」单价不能为负`)
			return null
		}
		map[spec] = row.price
		if (row.price > 0) hasPositive = true
	}
	if (!hasPositive) {
		Message.warning('按秒计费至少配置一档正价（建议额外配置 * 兜底价）')
		return null
	}
	return map
}

// 时段预览基准价：优先 * 兜底价，否则矩阵最低正价
function perSecondBasePrice(): number {
	const prices = perSecondRows.map((r) => Number(r.price) || 0).filter((p) => p > 0)
	if (prices.length === 0) return 0
	const wildcard = perSecondRows.find((r) => r.spec.trim() === '*')
	return wildcard && wildcard.price > 0 ? wildcard.price : Math.min(...prices)
}

function resetEditorDefaults() {
	editorBillingMode.value = 'token'
	editorItems.length = 0
	editorItems.push(createEmptyItem('token'))
	perSecondRows.length = 0
	paramRules.length = 0
	tryMatchInput.value = ''
	editorPriceNote.value = ''
	editorDiscountLabel.value = ''
	editorPriceChangeNote.value = ''
	resetTimeSegments()
}

// ============================================================
// Official Pricing Fetch
// ============================================================
const officialLoading = ref(false)
const officialData = ref<any>(null)
const officialError = ref('')
const officialPricingVisible = ref(false)

const officialSourceMeta: Record<string, { label: string; color: string }> = {
	litellm: { label: 'LiteLLM', color: 'arcoblue' },
	'models.dev': { label: 'models.dev', color: 'green' },
	basellm: { label: 'BaseLLM', color: 'orange' },
	openrouter: { label: 'OpenRouter', color: 'purple' },
}

function getOfficialSourceMeta(source: string) {
	return officialSourceMeta[source] || { label: source, color: 'gray' }
}

// 官方数据源价格恒为美元；本位币=CNY 时按汇率倒数折算展示与填入（汇率恒为 CNY→USD 单向配置，USD→CNY 取倒数）
const usdToBaseRatio = computed(() => {
	if (displayCurrency.value !== 'CNY') return 1
	return 1 / cnyToUsd.value
})

// USD 原值 → 本位币值（4 位小数，与定价输入控件精度对齐）
function toBasePrice(v: number | null | undefined): number {
	const usd = Number(v ?? 0)
	if (usd <= 0) return 0
	return Math.round(usd * usdToBaseRatio.value * 10000) / 10000
}

// 官方参考价展示：折算为本位币直显；本位币=CNY 时括注美元原值便于核对
function formatOfficialPrice(v: number | null | undefined): string {
	const converted = formatBilling(toBasePrice(v), 2)
	if (displayCurrency.value !== 'CNY') return converted
	const usd = Number(Number(v ?? 0).toFixed(2))
	return usd > 0 ? `${converted} ($${usd})` : converted
}

function resetOfficialData() {
	officialData.value = null
	officialError.value = ''
	officialPricingVisible.value = false
}

function handleOfficialPricingVisibleChange(visible: boolean) {
	officialPricingVisible.value = visible
	if (visible && !officialData.value && !officialLoading.value) {
		fetchOfficialPricing()
	}
}

async function fetchOfficialPricing() {
	if (!props.modelId) return
	officialLoading.value = true
	officialError.value = ''
	officialData.value = null
	try {
		const res: any = await request.get(`/admin/models/${props.modelId}/official-pricing`)
		officialData.value = res.data?.data
	} catch {
		officialError.value = '获取官方定价失败'
	} finally {
		officialLoading.value = false
	}
}

function applyOfficialPricing(pricing: any) {
	if (!pricing) return
	if (editorItems.length > 0) {
		// 编辑表单按本位币原值输入，官方 USD 价格须先折算为本位币
		editorItems[0].input_price = toBasePrice(pricing.input_price)
		editorItems[0].output_price = toBasePrice(pricing.output_price)
		editorItems[0].cache_read_price = toBasePrice(pricing.cache_read_price)
		editorItems[0].cache_creation_price = toBasePrice(pricing.cache_creation_price)
	}
	if (pricing.billing_mode && pricing.billing_mode !== editorBillingMode.value) {
		editorBillingMode.value = pricing.billing_mode
	}
	officialPricingVisible.value = false
	Message.success('已填入官方参考定价')
}

// ============================================================
// Param Multipliers（参数倍率：按归一化任务体参数匹配，命中规则连乘）
// ============================================================
interface ParamConditionRow {
	path: string
	match: string // has_value | equals | gt | gte | lt | lte | between
	value: string // 输入态字符串；between 逗号分隔；提交时按匹配方式转换
}
interface ParamRuleRow {
	conditions: ParamConditionRow[]
	multiplier: number
	note: string
}

const paramRules = reactive<ParamRuleRow[]>([])

const paramMatchOptions = [
	{ label: '有值', value: 'has_value' },
	{ label: '等于', value: 'equals' },
	{ label: '大于', value: 'gt' },
	{ label: '大于等于', value: 'gte' },
	{ label: '小于', value: 'lt' },
	{ label: '小于等于', value: 'lte' },
	{ label: '区间', value: 'between' },
]

// 常用路径预设（归一化任务体，规则跟语义走不跟入口协议走）
const paramPathPresets = [
	'metadata.image',
	'metadata.duration',
	'metadata.resolution',
	'metadata.aspect_ratio',
	'seconds',
	'metadata.content[*].video_url',
	'metadata.content[*].role',
]

function addParamRule() {
	paramRules.push({ conditions: [{ path: '', match: 'has_value', value: '' }], multiplier: 1, note: '' })
}

function removeParamRule(index: number) {
	paramRules.splice(index, 1)
}

function addParamCondition(ruleIndex: number) {
	paramRules[ruleIndex].conditions.push({ path: '', match: 'has_value', value: '' })
}

function removeParamCondition(ruleIndex: number, condIndex: number) {
	paramRules[ruleIndex].conditions.splice(condIndex, 1)
}

function conditionNeedsValue(match: string): boolean {
	return match !== 'has_value'
}

// 条件输入态 → 提交值（数字类强转、between 转双元素数组、equals 保原样）
function conditionValuePayload(cond: ParamConditionRow): any {
	const raw = cond.value.trim()
	if (cond.match === 'between') {
		const parts = raw.split(',').map((s) => s.trim()).filter(Boolean)
		return parts.length === 2 ? parts.map(Number) : null
	}
	if (['gt', 'gte', 'lt', 'lte'].includes(cond.match)) {
		const n = Number(raw)
		return raw !== '' && !Number.isNaN(n) ? n : null
	}
	return raw // equals：字符串原样（后端数字强转比较）
}

// 行编辑态 → 提交载荷；校验失败返回 null
function buildParamMultipliersPayload(): any[] | null {
	const payload: any[] = []
	for (const [i, rule] of paramRules.entries()) {
		if (rule.multiplier <= 0 || rule.multiplier > 100) {
			Message.warning(`参数倍率规则 ${i + 1} 倍率需在 (0, 100] 区间`)
			return null
		}
		if (rule.conditions.length === 0) {
			Message.warning(`参数倍率规则 ${i + 1} 至少需要一个条件`)
			return null
		}
		const conditions: any[] = []
		for (const [j, cond] of rule.conditions.entries()) {
			const path = cond.path.trim()
			if (!path) {
				Message.warning(`参数倍率规则 ${i + 1} 条件 ${j + 1} 路径为空`)
				return null
			}
			const value = conditionValuePayload(cond)
			if (conditionNeedsValue(cond.match) && (value === null || value === '')) {
				Message.warning(`参数倍率规则 ${i + 1} 条件 ${j + 1} 需要填写比对值`)
				return null
			}
			conditions.push(conditionNeedsValue(cond.match) ? { path, match: cond.match, value } : { path, match: cond.match })
		}
		// 命中说明：空 = 默认「规则N」（随序号，删除上方规则后自动顺延）
		payload.push({ conditions, multiplier: rule.multiplier, note: rule.note.trim() || `规则 ${i + 1}` })
	}
	return payload
}

// 回显：后端规则 → 行编辑态
function loadParamMultipliers(list: any[] | null | undefined) {
	paramRules.length = 0
	if (!Array.isArray(list)) return
	for (const rule of list) {
		const row: ParamRuleRow = {
			conditions: (rule.conditions || []).map((c: any) => ({
				path: c.path || '',
				match: c.match || 'has_value',
				value: Array.isArray(c.value) ? c.value.join(',') : c.value != null ? String(c.value) : '',
			})),
			multiplier: Number(rule.multiplier) || 1,
			note: rule.note || '',
		}
		if (row.conditions.length > 0) paramRules.push(row)
	}
}

// ── 试匹配预览（前端复刻后端求值语义：路径提取 + [*] 扇出 + 数字强转 + 隐式与 + 连乘） ──
const tryMatchInput = ref('')

function lookupPath(root: any, path: string): { values: any[]; found: boolean } {
	const segments = path.split('.').filter(Boolean)
	let current = [root]
	for (const seg of segments) {
		const wildcard = seg.endsWith('[*]')
		const key = wildcard ? seg.slice(0, -3) : seg
		const next: any[] = []
		for (const cur of current) {
			if (cur == null || typeof cur !== 'object' || Array.isArray(cur)) continue
			const v = (cur as Record<string, any>)[key]
			if (v === undefined) continue
			if (wildcard) {
				if (Array.isArray(v)) next.push(...v)
			} else {
				next.push(v)
			}
		}
		if (next.length === 0) return { values: [], found: false }
		current = next
	}
	return { values: current, found: true }
}

function toNum(v: any): number | null {
	if (typeof v === 'number') return v
	if (typeof v === 'string' && v.trim() !== '' && !Number.isNaN(Number(v))) return Number(v)
	return null
}

function conditionHit(cond: { path: string; match: string; value?: any }, root: any): boolean {
	const { values, found } = lookupPath(root, cond.path)
	if (!found) return false
	if (cond.match === 'has_value') return values.some((v) => v !== null && v !== undefined)
	const target = Array.isArray(cond.value) ? cond.value.map(Number) : cond.value
	if (cond.match === 'equals') {
		return values.some((v) => {
			const vn = toNum(v)
			const tn = toNum(target)
			if (vn != null && tn != null) return vn === tn
			return v === target
		})
	}
	if (cond.match === 'between') {
		if (!Array.isArray(target) || target.length !== 2) return false
		return values.some((v) => {
			const n = toNum(v)
			return n != null && n >= target[0] && n <= target[1]
		})
	}
	const tn = toNum(target)
	if (tn == null) return false
	return values.some((v) => {
		const n = toNum(v)
		if (n == null) return false
		if (cond.match === 'gt') return n > tn
		if (cond.match === 'gte') return n >= tn
		if (cond.match === 'lt') return n < tn
		return n <= tn // lte
	})
}

// 试匹配结果：命中规则列表 + 总倍率
function tryMatchResult(): { hits: Array<{ label: string; multiplier: number }>; total: number } | null {
	const raw = tryMatchInput.value.trim()
	if (!raw) return null
	let root: any
	try {
		root = JSON.parse(raw)
	} catch {
		return null
	}
	const hits: Array<{ label: string; multiplier: number }> = []
	let total = 1
	for (const rule of paramRules) {
		const allHit = rule.conditions.every((c) =>
			conditionHit({ path: c.path, match: c.match, value: conditionValuePayload(c) }, root),
		)
		if (allHit) {
			total *= rule.multiplier || 1
			hits.push({ label: rule.note || rule.conditions.map((c) => c.path).join(' & '), multiplier: rule.multiplier })
		}
	}
	return { hits, total }
}

// ============================================================
// Load Pricing Data
// ============================================================
async function loadPricing() {
	if (!props.modelId) return
	editorItems.length = 0
	editorBillingMode.value = 'token'
	paramRules.length = 0
	tryMatchInput.value = ''
	resetOfficialData()
	editorLoading.value = true

	try {
		const res: any = await request.get(`/admin/models/${props.modelId}/pricing`)
		const list: any[] = res.data?.data?.list || []
		editorPriceNote.value = res.data?.data?.price_note || ''
		editorDiscountLabel.value = res.data?.data?.discount_label || ''
		editorPriceChangeNote.value = res.data?.data?.price_change_note || ''
		if (list.length > 0) {
			editorBillingMode.value = list[0].billing_mode || 'token'
			editorItems.length = 0
			for (const item of list) {
				editorItems.push({
					billing_mode: item.billing_mode,
					min_tokens: item.min_tokens || 0,
					max_tokens: item.max_tokens ?? null,
					input_price: item.input_price ?? 0,
					output_price: item.output_price ?? 0,
					per_request_price: item.per_request_price ?? null,
					cache_read_price: item.cache_read_price ?? 0,
					cache_creation_price: item.cache_creation_price ?? 0,
					per_second_prices: item.per_second_prices ?? null,
				})
			}
			loadPerSecondPrices(list[0].per_second_prices)
		} else {
			resetEditorDefaults()
		}
		loadTimeSegments(res.data?.data?.time_segments)
		loadParamMultipliers(res.data?.data?.param_multipliers)
	} catch {
		resetEditorDefaults()
	} finally {
		editorLoading.value = false
	}
}

// ============================================================
// Time Segment Pricing Editor（时段定价：按命名时段对全部价格等比缩放）
// ============================================================
interface TimeSegmentRow {
	name: string
	dayPreset: string // every / weekday / weekend / custom
	days: number[] // 仅 custom 使用，1=周一..7=周日
	start_time: string // HH:mm，与 end_time 均空=全天
	end_time: string
	validRange: string[] | undefined // 有效期 [from, to]，可选（促销/定时调价）
	percent: number // 乘数百分比，50 = 半价（提交时换算为 multiplier）
}

const timePricingEnabled = ref(false)
const editorTimeSegments = reactive<TimeSegmentRow[]>([])

const dayPresetOptions = [
	{ label: '每天', value: 'every' },
	{ label: '工作日', value: 'weekday' },
	{ label: '周末', value: 'weekend' },
	{ label: '自定义', value: 'custom' },
]

const weekdayOptions = [
	{ label: '周一', value: 1 },
	{ label: '周二', value: 2 },
	{ label: '周三', value: 3 },
	{ label: '周四', value: 4 },
	{ label: '周五', value: 5 },
	{ label: '周六', value: 6 },
	{ label: '周日', value: 7 },
]

// 预设 → days 数组（提交时换算；every=空数组表示每天）
const presetToDays: Record<string, number[]> = {
	every: [],
	weekday: [1, 2, 3, 4, 5],
	weekend: [6, 7],
}

// days 数组 → 预设（回显时换算）
function daysToPreset(days: number[] | null | undefined): string {
	if (!days || days.length === 0) return 'every'
	const sorted = [...days].sort((a, b) => a - b)
	if (sorted.join(',') === '1,2,3,4,5') return 'weekday'
	if (sorted.join(',') === '6,7') return 'weekend'
	return 'custom'
}

function createEmptySegment(): TimeSegmentRow {
	return { name: '', dayPreset: 'every', days: [], start_time: '', end_time: '', validRange: undefined, percent: 100 }
}

function addTimeSegment() {
	editorTimeSegments.push(createEmptySegment())
}

function removeTimeSegment(index: number) {
	editorTimeSegments.splice(index, 1)
}

// 上移/下移：时段从上到下先命中先生效，促销时段应排在常驻时段之前
function moveTimeSegment(index: number, offset: number) {
	const target = index + offset
	if (target < 0 || target >= editorTimeSegments.length) return
	const [seg] = editorTimeSegments.splice(index, 1)
	editorTimeSegments.splice(target, 0, seg)
}

// 组合预览基础价：随计费模式取当前编辑中的价格（tiered 用首档价）
const previewBase = computed(() => {
	const first: any = editorItems[0] || {}
	if (editorBillingMode.value === 'per_second') {
		return { kind: 'per_second' as const, perSecond: perSecondBasePrice() }
	}
	if (editorBillingMode.value === 'per_request') {
		return { kind: 'per_request' as const, perRequest: Number(first.per_request_price) || 0 }
	}
	const kind = editorBillingMode.value === 'tiered' ? ('tiered' as const) : ('token' as const)
	return { kind, input: Number(first.input_price) || 0, output: Number(first.output_price) || 0 }
})

function segmentPreview(seg: TimeSegmentRow): string {
	const m = (seg.percent || 0) / 100
	if (previewBase.value.kind === 'per_second') {
		return `${formatBilling(previewBase.value.perSecond * m, 4)} /秒`
	}
	if (previewBase.value.kind === 'per_request') {
		return `${formatBilling(previewBase.value.perRequest * m, 4)} /次`
	}
	const prefix = previewBase.value.kind === 'tiered' ? '首档 ' : ''
	return `${prefix}输入 ${formatBilling(previewBase.value.input * m, 4)} · 输出 ${formatBilling(previewBase.value.output * m, 4)}`
}

function defaultPreviewText(): string {
	if (previewBase.value.kind === 'per_second') {
		return `${formatBilling(previewBase.value.perSecond, 4)} /秒`
	}
	if (previewBase.value.kind === 'per_request') {
		return `${formatBilling(previewBase.value.perRequest, 4)} /次`
	}
	const prefix = previewBase.value.kind === 'tiered' ? '首档 ' : ''
	return `${prefix}输入 ${formatBilling(previewBase.value.input, 4)} · 输出 ${formatBilling(previewBase.value.output, 4)}`
}

function segmentDesc(seg: TimeSegmentRow): string {
	const daysText =
		seg.dayPreset === 'custom'
			? seg.days.length
				? [...seg.days]
						.sort((a, b) => a - b)
						.map((d) => '周' + '一二三四五六日'[d - 1])
						.join('、')
				: '每天'
			: dayPresetOptions.find((o) => o.value === seg.dayPreset)?.label || '每天'
	const timeText = seg.start_time && seg.end_time ? `${seg.start_time}~${seg.end_time}` : '全天'
	let rangeText = ''
	if (seg.validRange?.[0]) {
		rangeText = seg.validRange[1] ? `，${seg.validRange[0]}~${seg.validRange[1]}` : `，${seg.validRange[0]} 起`
	}
	return `${daysText} ${timeText}${rangeText}`
}

// 提交载荷：percent → multiplier，预设 → days
function segmentsPayload(): any[] {
	if (!timePricingEnabled.value) return []
	return editorTimeSegments.map((seg) => ({
		name: seg.name.trim(),
		days: seg.dayPreset === 'custom' ? [...seg.days].sort((a, b) => a - b) : presetToDays[seg.dayPreset] || [],
		start_time: seg.start_time || '',
		end_time: seg.end_time || '',
		valid_from: seg.validRange?.[0] || '',
		valid_to: seg.validRange?.[1] || '',
		multiplier: (seg.percent || 0) / 100,
	}))
}

// 回显：multiplier → percent，days → 预设
function loadTimeSegments(list: any[] | null | undefined) {
	editorTimeSegments.length = 0
	timePricingEnabled.value = Array.isArray(list) && list.length > 0
	if (!timePricingEnabled.value) return
	for (const seg of list || []) {
		editorTimeSegments.push({
			name: seg.name || '',
			dayPreset: daysToPreset(seg.days),
			days: Array.isArray(seg.days) ? [...seg.days] : [],
			start_time: seg.start_time || '',
			end_time: seg.end_time || '',
			validRange:
				seg.valid_from || seg.valid_to ? [seg.valid_from || '', seg.valid_to || ''] : undefined,
			percent: Math.round((Number(seg.multiplier) || 1) * 10000) / 100,
		})
	}
}

function resetTimeSegments() {
	timePricingEnabled.value = false
	editorTimeSegments.length = 0
}

// ============================================================
// Tiered Mode Helpers
// ============================================================
watch(editorBillingMode, (mode) => {
	if (mode === 'tiered') {
		if (editorItems.length === 0) {
			editorItems.push(createEmptyItem('tiered'))
		}
	} else {
		if (editorItems.length === 0) {
			editorItems.push(createEmptyItem(mode))
		} else {
			editorItems[0].billing_mode = mode
		}
	}
	// 切到按秒模式时给一个常见起始模板（已手动编辑过则不覆盖）
	if (mode === 'per_second' && perSecondRows.length === 0) {
		addPerSecondRow('720p')
		addPerSecondRow('*')
	}
})

function addTier() {
	const last = editorItems[editorItems.length - 1]
	const newMin = last?.max_tokens ?? 0
	editorItems.push(createEmptyItem('tiered'))
	const newItem = editorItems[editorItems.length - 1]
	newItem.min_tokens = newMin
	if (last && last.max_tokens === null) {
		last.max_tokens = newMin
	}
}

function removeTier(index: number) {
	editorItems.splice(index, 1)
	if (editorItems.length > 0) {
		editorItems[editorItems.length - 1].max_tokens = null
	}
}

// ============================================================
// Save
// ============================================================
async function savePricing() {
	if (!props.modelId) return
	// 时段定价前置校验（完整校验由后端 ValidateTimeSegments 兜底）
	if (timePricingEnabled.value) {
		if (editorTimeSegments.length === 0) {
			Message.warning('已开启时段定价，请至少添加一条时段')
			return
		}
		for (const [i, seg] of editorTimeSegments.entries()) {
			if (!seg.name.trim()) {
				Message.warning(`第 ${i + 1} 条时段名称不能为空`)
				return
			}
			if (!seg.percent || seg.percent <= 0) {
				Message.warning(`时段「${seg.name}」乘数必须大于 0`)
				return
			}
			if ((seg.start_time ? 1 : 0) !== (seg.end_time ? 1 : 0)) {
				Message.warning(`时段「${seg.name}」开始与结束时间需同时填写或同时留空`)
				return
			}
			if (seg.dayPreset === 'custom' && seg.days.length === 0) {
				Message.warning(`时段「${seg.name}」自定义适用日至少选择一天`)
				return
			}
		}
	}
	editorSaving.value = true
	try {
		// 按秒模式：行编辑器 → 矩阵（校验失败中止保存）
		let perSecondPrices: Record<string, number> | null = null
		if (editorBillingMode.value === 'per_second') {
			perSecondPrices = buildPerSecondPrices()
			if (!perSecondPrices) {
				editorSaving.value = false
				return
			}
		}
		// 参数倍率：行编辑器 → 提交载荷（校验失败中止保存）
		const paramMultipliers = buildParamMultipliersPayload()
		if (paramMultipliers === null) {
			editorSaving.value = false
			return
		}
		const items = editorItems.map((item, index) => ({
			billing_mode: editorBillingMode.value,
			min_tokens: editorBillingMode.value === 'tiered' ? item.min_tokens : 0,
			max_tokens: editorBillingMode.value === 'tiered' ? (index < editorItems.length - 1 ? item.max_tokens : null) : null,
			input_price: item.input_price,
			output_price: item.output_price,
			per_request_price: editorBillingMode.value === 'per_request' ? (item.per_request_price ?? null) : null,
			cache_read_price: item.cache_read_price,
			cache_creation_price: item.cache_creation_price,
			per_second_prices: editorBillingMode.value === 'per_second' ? perSecondPrices : null,
		}))
		await request.put(`/admin/models/${props.modelId}/pricing`, {
			items,
			time_segments: segmentsPayload(),
			param_multipliers: paramMultipliers,
			price_note: editorPriceNote.value.trim(),
			discount_label: editorDiscountLabel.value.trim(),
			price_change_note: editorPriceChangeNote.value.trim(),
		})
		Message.success('定价已保存')
		emit('update:visible', false)
		emit('saved')
	} catch {
		// error handled by interceptor
	} finally {
		editorSaving.value = false
	}
}

// ============================================================
// Watchers
// ============================================================
// 打开时自动加载定价数据
watch(() => props.visible, (val) => {
	if (val && props.modelId) {
		loadPricing()
	}
})

// 关闭时重置状态
watch(() => props.visible, (val) => {
	if (!val) {
		resetEditorDefaults()
		resetOfficialData()
	}
})
</script>

<template>
	<AModal
		:visible="visible"
		:title="modelId ? `定价设置 - ${modelIdStr}` : '定价设置'"
		:width="'min(880px, 96vw)'"
		:mask-closable="false"
		:esc-to-close="false"
		:footer="true"
		:body-style="{ maxHeight: '72vh', overflowY: 'auto' }"
		@cancel="emit('update:visible', false)"
	>
		<template #footer>
			<AButton @click="emit('update:visible', false)">取消</AButton>
			<AButton type="primary" :loading="editorSaving" @click="savePricing">保存定价</AButton>
		</template>
		<ASpin :loading="editorLoading" style="width: 100%">
		<AForm :model="{}" layout="vertical">
			<div class="pricing-editor">
				<!-- Billing Mode -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>计费模式</h3>
					</div>
					<div class="billing-mode-row">
						<ARadioGroup v-model="editorBillingMode" type="button">
							<ARadio v-for="opt in billingModeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
						</ARadioGroup>
						<APopover
							trigger="click"
							position="br"
							content-class="official-reference-popup"
							:popup-visible="officialPricingVisible"
							@popup-visible-change="handleOfficialPricingVisibleChange"
						>
							<AButton size="small" type="outline" :loading="officialLoading">
								<template #icon><IconCloudDownload /></template>
								官方参考价
							</AButton>
							<template #content>
								<div class="official-reference-panel">
									<div class="official-reference-header">
										<div>
											<div class="official-reference-title">官方参考定价</div>
											<div class="official-reference-model">{{ officialData?.model_name || modelIdStr }}</div>
										</div>
										<AButton size="mini" :loading="officialLoading" @click="fetchOfficialPricing">
											<template #icon><IconRefresh /></template>
											重新拉取
										</AButton>
									</div>

									<div v-if="officialLoading && !officialData" class="official-reference-state">
										<ASpin />
									</div>
									<AAlert v-else-if="officialError" type="error" :title="officialError" />
									<template v-else-if="officialData">
										<div v-if="displayCurrency === 'CNY'" class="official-reference-hint">
											数据源价格单位为美元，已按 1 USD ≈ ¥{{ (1 / cnyToUsd).toFixed(4) }} 折算为人民币，括号内为美元原值
										</div>
										<div class="official-source-grid">
											<div v-for="src in officialData.sources" :key="src.source" class="official-source-card">
												<div class="official-source-header">
													<ATag size="small" :color="getOfficialSourceMeta(src.source).color">
														{{ getOfficialSourceMeta(src.source).label }}
													</ATag>
													<template v-if="src.found && src.pricing">
														<ATag v-if="src.provider" size="small" color="orangered">{{ src.provider }}</ATag>
														<ATag v-if="src.mode" size="small">{{ src.mode }}</ATag>
													</template>
												</div>
												<template v-if="src.found && src.pricing">
													<div v-if="src.max_context_tokens || src.max_output_tokens" class="official-source-meta">
														<span v-if="src.max_context_tokens">上下文 {{ (src.max_context_tokens / 1000).toFixed(0) }}K</span>
														<span v-if="src.max_output_tokens">输出 {{ (src.max_output_tokens / 1000).toFixed(0) }}K</span>
													</div>
													<div class="official-prices">
														<div class="official-price-row">
															<span class="official-price-label">输入价格</span>
															<span class="official-price-value">{{ formatOfficialPrice(src.pricing.input_price) }} / 1M</span>
														</div>
														<div class="official-price-row">
															<span class="official-price-label">输出价格</span>
															<span class="official-price-value">{{ formatOfficialPrice(src.pricing.output_price) }} / 1M</span>
														</div>
														<div class="official-price-row">
															<span class="official-price-label">缓存读取</span>
															<span class="official-price-value">{{ src.pricing.cache_read_price ? formatOfficialPrice(src.pricing.cache_read_price) + ' / 1M' : '—' }}</span>
														</div>
														<div class="official-price-row">
															<span class="official-price-label">缓存创建</span>
															<span class="official-price-value">{{ src.pricing.cache_creation_price ? formatOfficialPrice(src.pricing.cache_creation_price) + ' / 1M' : '—' }}</span>
														</div>
													</div>
													<AButton type="primary" size="small" long @click="applyOfficialPricing(src.pricing)">应用此价格</AButton>
												</template>
												<div v-else class="official-not-found" :class="{ 'official-not-found--error': src.error }">
													{{ src.error ? '数据源暂时不可用' : '未找到该模型' }}
												</div>
											</div>
										</div>
									</template>
								</div>
							</template>
						</APopover>
					</div>
				</div>

				<!-- Per-request Mode -->
				<template v-if="editorBillingMode === 'per_request' && editorItems.length > 0">
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>按次定价</h3>
						</div>
						<div class="grid grid-cols-2 gap-x-4">
							<AFormItem label="按次单价">
								<AInputNumber
									v-model="editorItems[0].per_request_price"
									:min="0"
									:precision="4"
									placeholder="每次调用价格"
									class="w-full"
								>
									<template #suffix>{{ currencySymbol }} / 次</template>
								</AInputNumber>
							</AFormItem>
						</div>
					</div>
				</template>

				<!-- Per-second Mode（视频按秒计费：分辨率 × 每秒单价矩阵，单卡片行式紧凑布局） -->
				<template v-else-if="editorBillingMode === 'per_second'">
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>按秒定价</h3>
							<span class="section-hint">按视频时长（秒）计费，费用 = 命中规格单价 × 秒数；<code>*</code> 为兜底价（请求规格未命中时使用）</span>
						</div>
						<div class="per-second-card">
							<div v-if="perSecondRows.length" class="per-second-col-head">
								<span>规格</span>
								<span>每秒单价</span>
								<span class="per-second-op-col"></span>
							</div>
							<div v-for="(row, index) in perSecondRows" :key="index" class="per-second-row">
								<AInput
									v-model="row.spec"
									placeholder="如 720p / 1080p / *（兜底）"
									:max-length="32"
									class="per-second-spec"
								/>
								<AInputNumber
									v-model="row.price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="per-second-price"
								>
									<template #suffix>{{ currencySymbol }} / 秒</template>
								</AInputNumber>
								<AButton
									size="mini"
									status="danger"
									title="删除该规格"
									@click="removePerSecondRow(index)"
								>✕</AButton>
							</div>
							<div v-if="!perSecondRows.length" class="per-second-empty">
								暂无规格，点击下方按钮或预设快捷添加
							</div>
						</div>
						<div class="flex items-center gap-2 mt-2">
							<AButton size="small" type="outline" @click="addPerSecondRow()">+ 添加规格</AButton>
							<AButton
								v-for="preset in perSecondSpecPresets"
								:key="preset"
								size="mini"
								@click="addPerSecondRow(preset)"
							>{{ preset }}</AButton>
						</div>
					</div>
				</template>

				<!-- Token Mode -->
				<template v-else-if="editorBillingMode === 'token' && editorItems.length > 0">
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>按量定价</h3>
							<span class="section-hint">价格单位为 {{ currencySymbol }} / 1M Token</span>
						</div>
						<div class="grid grid-cols-2 md:grid-cols-4 gap-x-4">
							<AFormItem label="输入价格">
								<AInputNumber
									v-model="editorItems[0].input_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>{{ currencySymbol }} / 1M</template>
								</AInputNumber>
							</AFormItem>
							<AFormItem label="输出价格">
								<AInputNumber
									v-model="editorItems[0].output_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>{{ currencySymbol }} / 1M</template>
								</AInputNumber>
							</AFormItem>
							<AFormItem label="缓存读取">
								<AInputNumber
									v-model="editorItems[0].cache_read_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>{{ currencySymbol }} / 1M</template>
								</AInputNumber>
							</AFormItem>
							<AFormItem label="缓存创建">
								<AInputNumber
									v-model="editorItems[0].cache_creation_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>{{ currencySymbol }} / 1M</template>
								</AInputNumber>
							</AFormItem>
						</div>
					</div>
				</template>

				<!-- Tiered Mode -->
				<template v-else>
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>阶梯定价</h3>
							<span class="section-hint">按 Token 用量分段设置不同价格</span>
						</div>
						<div v-for="(tier, index) in editorItems" :key="index" class="tier-card">
							<div class="tier-header">
								<span class="tier-label">第 {{ index + 1 }} 梯</span>
								<AButton
									v-if="editorItems.length > 1"
									size="mini"
									status="danger"
									@click="removeTier(index)"
								>删除</AButton>
							</div>
							<div class="grid grid-cols-2 md:grid-cols-4 gap-x-4">
								<AFormItem label="起始 Token">
									<AInputNumber
										v-model="tier.min_tokens"
										:min="0"
										:step="1000"
										placeholder="0"
										class="w-full"
									/>
								</AFormItem>
								<AFormItem label="结束 Token">
									<AInputNumber
										v-if="index < editorItems.length - 1"
										v-model="tier.max_tokens"
										:min="0"
										:step="1000"
										placeholder="上限"
										class="w-full"
									/>
									<AInput v-else model-value="无上限" disabled class="w-full" />
								</AFormItem>
								<AFormItem label="输入价格">
									<AInputNumber
										v-model="tier.input_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="输出价格">
									<AInputNumber
										v-model="tier.output_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
							</div>
							<div class="grid grid-cols-2 gap-x-4 mt-1">
								<AFormItem label="缓存读取价格">
									<AInputNumber
										v-model="tier.cache_read_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="缓存创建价格">
									<AInputNumber
										v-model="tier.cache_creation_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
							</div>
						</div>
						<AButton type="dashed" long @click="addTier" class="mt-3">+ 添加梯度</AButton>
					</div>
				</template>

				<!-- Time Segment Pricing（时段定价：峰谷/工作时段/促销） -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>时段定价</h3>
						<ASwitch v-model="timePricingEnabled" size="small" />
						<span class="section-hint">全部价格（含各档位/缓存/按次价）按乘数等比缩放：100% = 默认价，未命中时段按默认价</span>
					</div>
					<template v-if="timePricingEnabled">
						<div v-for="(seg, index) in editorTimeSegments" :key="index" class="tier-card">
							<div class="tier-header">
								<div class="segment-tools">
									<AButton size="mini" :disabled="index === 0" @click="moveTimeSegment(index, -1)">↑</AButton>
									<AButton size="mini" :disabled="index === editorTimeSegments.length - 1" @click="moveTimeSegment(index, 1)">↓</AButton>
									<span class="tier-label">第 {{ index + 1 }} 条</span>
								</div>
								<AButton size="mini" status="danger" @click="removeTimeSegment(index)">删除</AButton>
							</div>
							<div class="grid grid-cols-2 md:grid-cols-4 gap-x-4">
								<AFormItem label="时段名称">
									<AInput v-model="seg.name" placeholder="如：闲时 / 工作忙时" :maxlength="32" allow-clear />
								</AFormItem>
								<AFormItem label="价格乘数">
									<AInputNumber v-model="seg.percent" :min="1" :max="1000" :step="5" :precision="0" class="w-full">
										<template #suffix>%</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="每日时段（留空 = 全天）" class="md:col-span-2">
									<div class="time-range">
										<ATimePicker v-model="seg.start_time" format="HH:mm" allow-clear placeholder="开始" class="w-full" />
										<span class="time-sep">~</span>
										<ATimePicker v-model="seg.end_time" format="HH:mm" allow-clear placeholder="结束" class="w-full" />
									</div>
								</AFormItem>
							</div>
							<div class="grid grid-cols-1 md:grid-cols-2 gap-x-4 mt-1">
								<AFormItem label="适用日">
									<ARadioGroup v-model="seg.dayPreset" type="button" size="small">
										<ARadio v-for="opt in dayPresetOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
									</ARadioGroup>
								</AFormItem>
								<AFormItem label="有效期（促销/定时调价，可选）">
									<ARangePicker v-model="seg.validRange" class="w-full" allow-clear />
								</AFormItem>
								<AFormItem
									v-if="seg.dayPreset === 'custom'"
									label="自定义星期"
									class="custom-days-item md:col-span-2"
								>
									<div class="custom-days">
										<ACheckboxGroup v-model="seg.days" :options="weekdayOptions" />
									</div>
								</AFormItem>
							</div>
							<div class="segment-preview">{{ segmentDesc(seg) }} → {{ segmentPreview(seg) }}</div>
						</div>
						<AButton type="dashed" long class="mt-3" @click="addTimeSegment">+ 添加时段</AButton>
						<div class="combo-preview-hint">时段从上到下先命中先生效：限时促销时段应排在常驻时段上方</div>

						<!-- 组合预览：换算后的实际价格 -->
						<div v-if="editorTimeSegments.length" class="combo-preview">
							<div class="combo-preview-title">价格预览（换算后实际价格）</div>
							<div class="combo-row">
								<span class="combo-name">默认价（未命中时段）</span>
								<span class="combo-value">{{ defaultPreviewText() }}</span>
							</div>
							<div v-for="(seg, index) in editorTimeSegments" :key="index" class="combo-row">
								<span class="combo-name">{{ seg.name || `时段 ${index + 1}` }}（{{ segmentDesc(seg) }}）</span>
								<span class="combo-value">{{ segmentPreview(seg) }}</span>
							</div>
						</div>
					</template>
				</div>

				<!-- Param Multipliers（参数倍率：按请求参数匹配加/折扣，所有计费模式共享） -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>参数倍率</h3>
						<span class="section-hint">按归一化任务体参数匹配（如含参考图 / 时长档位 / 视频输入），命中规则连乘；所有计费模式共享</span>
					</div>
					<div v-for="(rule, ruleIndex) in paramRules" :key="ruleIndex" class="tier-card">
						<!-- 头行：命中说明（默认占位"规则N"）跨前两列 · 命中倍率对齐「值」列 · 删除列 -->
						<div class="param-rule-head">
							<div class="param-field param-field--note">
								<span class="field-label field-label--required">说明</span>
								<AInput
									v-model="rule.note"
									:placeholder="`规则 ${ruleIndex + 1}`"
									:max-length="64"
									allow-clear
									class="param-note-input"
								/>
							</div>
							<div class="param-field param-field--mult">
								<span class="field-label field-label--required">倍率</span>
								<AInputNumber
									v-model="rule.multiplier"
									:min="0.0001"
									:max="100"
									:precision="4"
									title="命中倍率"
									class="param-multiplier-input"
								>
									<template #suffix>×</template>
								</AInputNumber>
							</div>
							<!-- 删除整条规则：垃圾桶图标，与下方条件行的「−」同宽同列 -->
							<AButton
								class="param-head-remove"
								size="mini"
								status="danger"
								title="删除该规则"
								@click="removeParamRule(ruleIndex)"
							>
								<IconDelete />
							</AButton>
						</div>
						<!-- 条件行：路径 / 匹配 / 值 / 删除 四列固定网格；「值」列恒占位（无需比对值时留空），
						     各条件行纵向对齐，不随匹配方式增减列而跳动 -->
						<div
							v-for="(cond, condIndex) in rule.conditions"
							:key="condIndex"
							class="param-condition-grid"
						>
							<div class="param-field param-field--path">
								<span class="field-label field-label--required">路径</span>
								<ASelect
									v-model="cond.path"
									:options="paramPathPresets.map((p) => ({ label: p, value: p }))"
									allow-create
									allow-search
									placeholder="如 metadata.image"
									class="param-path-select"
								/>
							</div>
							<div class="param-field param-field--match">
								<span class="field-label field-label--required">匹配</span>
								<ASelect v-model="cond.match" :options="paramMatchOptions" class="param-match-select" />
							</div>
							<div
								class="param-field param-field--value"
								:class="{ 'is-empty': !conditionNeedsValue(cond.match) }"
							>
								<span class="field-label field-label--required">值</span>
								<AInput
									v-model="cond.value"
									:placeholder="cond.match === 'between' ? 'min,max（如 8,12）' : '比对值'"
									class="param-value-input"
								/>
							</div>
							<!-- 行尾「−」= 移除该条件（仅剩一条时禁用）；不用 ✕ 避免被当成关闭弹窗 -->
							<AButton
								class="param-cond-remove"
								size="mini"
								status="danger"
								title="移除该条件"
								:disabled="rule.conditions.length <= 1"
								@click="removeParamCondition(ruleIndex, condIndex)"
							>−</AButton>
						</div>
						<AButton size="mini" type="text" @click="addParamCondition(ruleIndex)">+ 添加条件（全部满足才命中）</AButton>
					</div>
					<AButton type="dashed" long class="mt-3" @click="addParamRule">+ 添加规则</AButton>

					<!-- 试匹配：粘贴任务体 JSON 即时验证规则命中 -->
					<div v-if="paramRules.length" class="param-try-match">
						<div class="combo-preview-title">试匹配（粘贴归一化任务体 JSON 验证命中）</div>
						<ATextarea
							v-model="tryMatchInput"
							:auto-size="{ minRows: 3, maxRows: 8 }"
							placeholder='如 {"model":"...","prompt":"...","seconds":"8","metadata":{"image":"https://...","duration":8}}'
						/>
						<template v-if="tryMatchResult()">
							<div class="combo-row">
								<span class="combo-name">命中规则</span>
								<span class="combo-value">
									<template v-if="tryMatchResult()!.hits.length">
										<ATag v-for="(h, i) in tryMatchResult()!.hits" :key="i" size="small" color="arcoblue">
											{{ h.label }} ×{{ h.multiplier }}
										</ATag>
									</template>
									<template v-else>无（按原价）</template>
								</span>
							</div>
							<div class="combo-row">
								<span class="combo-name">总倍率</span>
								<span class="combo-value">×{{ tryMatchResult()!.total }}</span>
							</div>
						</template>
						<div v-else-if="tryMatchInput.trim()" class="combo-preview-hint">JSON 解析失败或为空</div>
					</div>
				</div>

				<!-- 展示信息（折扣标签 / 价格调整说明对外展示，价格说明仅内部可见） -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>展示信息</h3>
						<span class="section-hint">折扣标签与调整说明会展示给租户；价格说明仅管理后台可见</span>
					</div>
					<div class="grid grid-cols-1 md:grid-cols-2 gap-x-4">
						<AFormItem label="折扣标签（对外展示）">
							<AInput v-model="editorDiscountLabel" :maxlength="50" placeholder="如：7折起、限时5折" allow-clear />
						</AFormItem>
						<AFormItem label="价格调整说明（对外展示）">
							<AInput v-model="editorPriceChangeNote" :maxlength="200" placeholder="如：9月1日起输入价下调 20%" allow-clear />
						</AFormItem>
					</div>
					<AFormItem label="价格说明（仅内部可见）">
						<ATextarea
							v-model="editorPriceNote"
							:max-length="500"
							:auto-size="{ minRows: 2, maxRows: 4 }"
							placeholder="调价背景、渠道成本等内部备注，不对外展示"
						/>
					</AFormItem>
				</div>

			</div>
		</AForm>
		</ASpin>
	</AModal>
</template>

<style scoped>
.editor-section {
	margin-bottom: 24px;
}

.editor-section-header {
	display: flex;
	align-items: center;
	gap: 12px;
	margin-bottom: 12px;
}

.editor-section-header h3 {
	font-size: 14px;
	font-weight: 600;
	color: var(--ta-text-primary);
	margin: 0;
}

.section-hint {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.billing-mode-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	flex-wrap: wrap;
}

.tier-card {
	padding: 12px 16px;
	background: var(--color-fill-1);
	border-radius: 8px;
	margin-bottom: 8px;
}

.tier-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	margin-bottom: 8px;
}

.tier-label {
	font-size: 13px;
	font-weight: 600;
	color: var(--ta-text-secondary);
}

.segment-tools {
	display: flex;
	align-items: center;
	gap: 4px;
}

.time-range {
	display: flex;
	align-items: center;
	gap: 8px;
	width: 100%;
}

.time-sep {
	color: var(--ta-text-tertiary);
}

.custom-days-item {
	margin-top: -2px;
}

.custom-days {
	box-sizing: border-box;
	width: 100%;
	padding: 10px 12px;
	background: var(--color-fill-2);
	border-radius: 6px;
}

.custom-days :deep(.arco-checkbox-group) {
	display: grid;
	grid-template-columns: repeat(7, minmax(0, 1fr));
	gap: 8px 12px;
	width: 100%;
}

@media (max-width: 767px) {
	.custom-days :deep(.arco-checkbox-group) {
		grid-template-columns: repeat(4, minmax(0, 1fr));
	}
}

.segment-preview {
	margin-top: 10px;
	padding: 6px 10px;
	background: var(--color-fill-2);
	border-radius: 6px;
	font-size: 12px;
	color: var(--ta-text-secondary);
}

.combo-preview-hint {
	margin-top: 8px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

/* 按秒定价：单卡片行式紧凑布局（规格 + 单价 + 删除一行排布） */
.per-second-card {
	padding: 10px 12px;
	background: var(--color-fill-1);
	border: 1px solid var(--ta-border-light);
	border-radius: 8px;
}

.per-second-col-head {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-bottom: 6px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.per-second-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.per-second-row + .per-second-row {
	margin-top: 6px;
}

.per-second-spec {
	flex: 1 1 40%;
	min-width: 0;
}

.per-second-price {
	flex: 1;
	min-width: 0;
}

.per-second-op-col {
	flex: 0 0 28px;
}

.per-second-empty {
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-align: center;
	padding: 6px 0;
}

/* 行内字段标签：灰色小字前置，紧凑布局下的字段语义说明 */
.field-label {
	flex: none;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	white-space: nowrap;
}

/* 必填标记：星号决定位在标签左侧（不占文本宽度），标签整体多留 8px，
   各行控件左边缘仍对齐；星号色对齐 Arco 表单必填符号 */
.field-label--required {
	position: relative;
	padding-left: 8px;
}

.field-label--required::before {
	content: '*';
	position: absolute;
	left: 0;
	color: rgb(var(--danger-6));
}

/* 参数倍率：规则头行与条件行共用同一套列轨道（路径 · 匹配 · 值 · 删除）。
   头行「说明」占「路径」列、「倍率」落在「值」列，中间隔开「匹配」列，
   两个输入框既与下方条件行纵向对齐，又不会挤在一起 */
.param-rule-head,
.param-condition-grid {
	/* 匹配列 152px = 32px 必填标签 + 8px 间隙 + 112px 下拉框：
	   下拉框内部固定吃掉 50px（边框 2 + 左右内边距 24 + 箭头区 24），剩 62px 文本区，
	   刚好容纳最长的「大于等于」四字（14px 字号 × 4 = 56px）不被省略号截断 */
	--param-cols: minmax(0, 1.2fr) 152px minmax(0, 1fr) auto;
	display: grid;
	grid-template-columns: var(--param-cols);
	align-items: center;
	gap: 8px;
	margin-bottom: 8px;
}

.param-rule-head {
	grid-template-areas: 'note . mult remove';
}

.param-condition-grid {
	grid-template-areas: 'path match value remove';
}

/* 行内字段：标签 + 控件（标签定宽，各行控件左边缘对齐） */
.param-field {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}

.param-field .field-label {
	min-width: 32px; /* 8px 必填星号 + 24px 两字标签，保证控件左边缘对齐 */
}

.param-field--note {
	grid-area: note;
}

.param-field--mult {
	grid-area: mult;
}

.param-field--path {
	grid-area: path;
}

.param-field--match {
	grid-area: match;
}

.param-field--value {
	grid-area: value;
}

/* 控件撑满所属列 */
.param-field .param-note-input,
.param-field .param-multiplier-input,
.param-field .param-path-select,
.param-field .param-match-select,
.param-field .param-value-input {
	flex: 1;
	min-width: 0;
}

/* 匹配方式无需比对值（如「有值」）：保留列宽占位，仅隐藏内容 */
.param-field--value.is-empty {
	visibility: hidden;
}

/* 头行删除与条件行移除都是图标按钮：定宽 + 去内边距，两者同宽、右边缘对齐 */
.param-head-remove,
.param-cond-remove {
	grid-area: remove;
	width: 32px;
	padding: 0;
}

/* 窄屏：头行说明独占首行、倍率下移；条件行路径独占首行，匹配 / 值下移，避免列被压扁 */
@media (max-width: 640px) {
	.param-rule-head,
	.param-condition-grid {
		--param-cols: 152px minmax(0, 1fr) auto;
		row-gap: 6px;
	}

	.param-rule-head {
		grid-template-areas:
			'note note remove'
			'mult mult mult';
	}

	.param-condition-grid {
		grid-template-areas:
			'path path remove'
			'match value value';
	}
}

.param-try-match {
	margin-top: 12px;
	padding: 10px 14px;
	background: var(--color-fill-1);
	border: 1px solid var(--ta-border-light);
	border-radius: 8px;
}

.combo-preview {
	margin-top: 10px;
	padding: 10px 14px;
	background: var(--color-fill-1);
	border: 1px solid var(--ta-border-light);
	border-radius: 8px;
}

.combo-preview-title {
	font-size: 12px;
	font-weight: 600;
	color: var(--ta-text-secondary);
	margin-bottom: 6px;
}

.combo-row {
	display: flex;
	align-items: baseline;
	justify-content: space-between;
	gap: 12px;
	padding: 3px 0;
	font-size: 12px;
}

.combo-name {
	color: var(--ta-text-secondary);
	flex-shrink: 0;
}

.combo-value {
	color: var(--ta-text-primary);
	font-family: monospace;
	text-align: right;
}

.official-reference-panel {
	width: min(680px, calc(100vw - 48px));
	max-height: min(560px, calc(100vh - 96px));
	overflow-y: auto;
	padding: 4px;
	background: var(--color-bg-3);
	color: var(--color-text-1);
	border-radius: 6px;
}

:global(.official-reference-popup) {
	padding: 10px !important;
	background: var(--color-bg-3) !important;
	border-color: var(--color-border) !important;
	box-shadow: 0 14px 36px rgba(15, 23, 42, 0.2) !important;
}

.official-reference-header {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: 16px;
	margin-bottom: 12px;
}

.official-reference-title {
	font-size: 14px;
	font-weight: 600;
	color: var(--ta-text-primary);
}

.official-reference-model {
	margin-top: 2px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	word-break: break-all;
}

.official-reference-state {
	display: flex;
	align-items: center;
	justify-content: center;
	min-height: 180px;
}

.official-reference-hint {
	margin-bottom: 10px;
	padding: 6px 10px;
	font-size: 12px;
	color: var(--ta-text-secondary);
	background: var(--color-fill-2);
	border-radius: 4px;
}

.official-source-grid {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 10px;
}

.official-source-card {
	display: flex;
	flex-direction: column;
	min-width: 0;
	padding: 12px;
	border: 1px solid var(--color-border);
	border-radius: 6px;
	background: var(--color-bg-1);
	box-shadow: var(--ta-shadow-card);
	transition: border-color var(--ta-duration-fast), box-shadow var(--ta-duration-fast);
}

.official-source-card:hover {
	border-color: var(--color-primary-4);
	box-shadow: var(--ta-shadow-hover);
}

.official-source-header {
	display: flex;
	align-items: center;
	gap: 6px;
	flex-wrap: wrap;
	min-height: 24px;
}

.official-source-meta {
	display: flex;
	gap: 12px;
	margin-top: 6px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.official-prices {
	margin: 8px 0 10px;
}

.official-price-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 4px 0;
}

.official-price-label {
	font-size: 13px;
	color: var(--ta-text-secondary);
}

.official-price-value {
	font-size: 13px;
	font-weight: 500;
	color: var(--ta-text-primary);
	font-family: monospace;
}

.official-not-found {
	display: flex;
	align-items: center;
	min-height: 76px;
	font-size: 13px;
	color: var(--ta-text-tertiary);
}

.official-not-found--error {
	color: rgb(var(--danger-6));
}

@media (max-width: 639px) {
	.official-reference-panel {
		width: calc(100vw - 40px);
	}

	.official-source-grid {
		grid-template-columns: 1fr;
	}
}
</style>
