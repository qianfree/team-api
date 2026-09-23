<script setup lang="ts">
import type { Component } from 'vue'
import { ref, reactive, watch, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconCloudDownload, IconRefresh } from '@arco-design/web-vue/es/icon'
import request from '@/utils/request'
import { displayCurrency, currencySymbol, formatBilling, cnyToUsd } from '@/composables/useCurrency'
import PricingTimeSegmentsEditor, {
	type TimeSegmentRow,
	type SegmentPreviewBase,
	timeSegmentRowsFromAPI,
	timeSegmentPayloadFrom,
} from './PricingTimeSegmentsEditor.vue'
import PricingParamRulesEditor, { type ParamRuleRow, paramRuleRowsFromAPI, paramRulePayloadFrom } from './PricingParamRulesEditor.vue'
import { availableSchemeOptions, resolvePricingSchemeEditor } from './pricingSchemeEditors'

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

// 计费面板模式：generic = 通用引擎表单；特殊方案名 = 专属编辑器（契约见 pricingSchemeEditors.ts）。
// 模式下拉位于 footer 左下角，特殊选项按模型编码前缀白名单过滤（仅特定模型可选）
const panelMode = ref<string>('generic')
const schemeConfigData = ref<any>(null)
const schemeEditorRef = ref<any>()

// 当前模型可用的特殊计费选项（模型名不命中任何前缀时为空，footer 不显示下拉）
const schemeOptions = computed(() => availableSchemeOptions(props.modelIdStr))

// 当前模式命中的特殊编辑器组件（generic 或未登记方案返回 null = 用通用表单）
const activeSchemeEditor = computed<Component | null>(() =>
	panelMode.value === 'generic' ? null : resolvePricingSchemeEditor(panelMode.value),
)

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

// 行 → 提交 map；requirePositive=true 要求至少一档正价（计费定价必需，官方参考价不强制）。
// map 为 null 表示校验失败（error 为中文提示文案）
function buildPerSecondMap(rows: PerSecondRow[], requirePositive: boolean): { map: Record<string, number> | null; error?: string } {
	const map: Record<string, number> = {}
	let hasPositive = false
	for (const row of rows) {
		const spec = row.spec.trim()
		if (!spec) continue
		if (map[spec] !== undefined) {
			return { map: null, error: `按秒计费规格「${spec}」重复` }
		}
		if (row.price < 0) {
			return { map: null, error: `按秒计费规格「${spec}」单价不能为负` }
		}
		map[spec] = row.price
		if (row.price > 0) hasPositive = true
	}
	if (requirePositive && !hasPositive) {
		return { map: null, error: '按秒计费至少配置一档正价（建议额外配置 * 兜底价）' }
	}
	return { map }
}

// 主定价按秒矩阵：校验失败 Message 提示并返回 null
function buildPerSecondPrices(): Record<string, number> | null {
	const result = buildPerSecondMap(perSecondRows, true)
	if (result.map === null) {
		if (result.error) Message.warning(result.error)
		return null
	}
	return result.map
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
	editorTimeSegments.length = 0
	timePricingEnabled.value = false
	paramEditorRef.value?.resetParamRules()
	editorPriceNote.value = ''
	editorDiscountLabel.value = ''
	editorPriceChangeNote.value = ''
	resetOfficialPricing()
	officialDiscountPercent.value = 100
}

// ============================================================
// Official Pricing Fetch（数据源参考价：官方价二层弹窗内懒加载）
// ============================================================
const officialLoading = ref(false)
const officialData = ref<any>(null)
const officialError = ref('')
const officialModalVisible = ref(false)

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

// USD 原值 → 本位币值（6 位小数，与资金显示精度及定价输入控件精度对齐）
function toBasePrice(v: number | null | undefined): number {
	const usd = Number(v ?? 0)
	if (usd <= 0) return 0
	return Math.round(usd * usdToBaseRatio.value * 1e6) / 1e6
}

// 金额按 6 位小数取整（折扣计算结果落编辑器前的唯一取整点）
function round6(v: number): number {
	return Math.round(v * 1e6) / 1e6
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
	officialModalVisible.value = false
}

// 打开官方价二层弹窗：数据源不自动拉取，用户在弹窗内按需展开（后端串行请求 4 个源，冷缓存较慢）
function openOfficialModal() {
	officialModalVisible.value = true
}

// 数据源参考价浮窗（APopover 挂在标题行按钮上）：trigger=click 非受控，
// 首次弹出自动拉取；已拉取过则直接展示
function onSourcesVisibleChange(visible: boolean) {
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

// ============================================================
// 官方定价（official_pricing 落库参照：非计费依据，是折扣计算与几折展示的基准）。
// 存储结构与计费定价完全同构（official_items + 时段 + 参数倍率，支持全部四种计费模式）；
// 数据源拉取仅作平面价初值参考，人工核对修正后随保存落库，调折扣不再依赖远程数据源
// ============================================================
const officialBillingMode = ref('token')
const officialItems = reactive<any[]>([])
const officialPerSecondRows = reactive<PerSecondRow[]>([])
const officialTimeSegments = reactive<TimeSegmentRow[]>([])
const officialTimeEnabled = ref(false)
const officialParamRules = reactive<ParamRuleRow[]>([])
// 官方时段/倍率的回显与载荷一律走纯函数填充/构建行数组，不依赖编辑器组件挂载
// （编辑器在二层弹窗内，Arco Modal 关闭即卸载，ref 不可靠）

const officialBillingModeOptions = [
	{ label: '按量 (Token)', value: 'token' },
	{ label: '按次', value: 'per_request' },
	{ label: '阶梯计费', value: 'tiered' },
	{ label: '按秒', value: 'per_second' },
]

// 官方定价首行（单行模式取价用；tiered 的档位价在 officialItems 各行）
function officialFirst(): any {
	return officialItems[0] || {}
}

// 官方定价是否已配置（价格维度或时段/倍率任一非空；阶梯看任一档——首档可能是 0 价免费档）
const hasOfficialPricing = computed(() => {
	if (officialTimeSegments.length > 0 || officialParamRules.length > 0) return true
	if (officialBillingMode.value === 'tiered') {
		return officialItems.some(
			(t) =>
				(Number(t.input_price) || 0) > 0 ||
				(Number(t.output_price) || 0) > 0 ||
				(Number(t.cache_read_price) || 0) > 0 ||
				(Number(t.cache_creation_price) || 0) > 0,
		)
	}
	const first = officialFirst()
	return (
		Number(first.input_price) > 0 ||
		Number(first.output_price) > 0 ||
		Number(first.cache_read_price) > 0 ||
		Number(first.cache_creation_price) > 0 ||
		(Number(first.per_request_price) || 0) > 0 ||
		officialPerSecondRows.some((r) => (Number(r.price) || 0) > 0)
	)
})

function addOfficialPerSecondRow(spec = '') {
	officialPerSecondRows.push({ spec, price: 0 })
}

function removeOfficialPerSecondRow(index: number) {
	officialPerSecondRows.splice(index, 1)
}

// 保证官方定价首行存在：模板直接绑定 officialItems[0]，空数组会渲染报错；
// 空行（价格全 0）不视为已配置，提交时后端按全空清除，无害
function ensureOfficialFirstRow(mode = officialBillingMode.value) {
	if (officialItems.length === 0) {
		officialItems.push({ billing_mode: mode, min_tokens: 0, max_tokens: null, input_price: 0, output_price: 0, per_request_price: null, cache_read_price: 0, cache_creation_price: 0, per_second_prices: null })
	}
}

// 切换官方计费模式时保证首行存在且行数合法（保留原首行价格，tiered 之外不保留多余档位行）
watch(officialBillingMode, (mode) => {
	ensureOfficialFirstRow(mode)
	if (mode !== 'tiered' && officialItems.length > 1) {
		officialItems.splice(1)
	}
	officialItems[0].billing_mode = mode
	// 切到按秒模式时给常见起始模板（与主编辑器一致，已手动填写则不覆盖）
	if (mode === 'per_second' && officialPerSecondRows.length === 0) {
		addOfficialPerSecondRow('720p')
		addOfficialPerSecondRow('*')
	}
})

// 组件创建即补首行（定价弹窗在模型列表页常驻渲染，此时 loadPricing 尚未执行）
ensureOfficialFirstRow()

function addOfficialTier() {
	// 阶梯档位行：上一档 max_tokens 收口为新增档的 min_tokens（与主弹窗阶梯语义一致）
	const last = officialItems[officialItems.length - 1]
	if (last && last.max_tokens != null) {
		const nextMin = Number(last.max_tokens) + 1
		last.max_tokens = nextMin - 1
		officialItems.push({ billing_mode: 'tiered', min_tokens: nextMin, max_tokens: null, input_price: 0, output_price: 0, per_request_price: null, cache_read_price: 0, cache_creation_price: 0, per_second_prices: null })
	} else {
		officialItems.push({ billing_mode: 'tiered', min_tokens: 0, max_tokens: null, input_price: 0, output_price: 0, per_request_price: null, cache_read_price: 0, cache_creation_price: 0, per_second_prices: null })
	}
}

function removeOfficialTier(index: number) {
	if (officialItems.length <= 1) return
	officialItems.splice(index, 1)
	// 末档保持无上限
	if (officialItems.length > 0) officialItems[officialItems.length - 1].max_tokens = null
}

// 回显：GET pricing 响应 → 官方定价编辑器（official_items + 时段 + 倍率，同构结构）。
// 时段/倍率用纯函数直接填充行数组——二层弹窗未打开时编辑器组件未挂载，ref 加载不生效
function loadOfficialPricing(data: any) {
	officialBillingMode.value = 'token'
	officialItems.length = 0
	officialPerSecondRows.length = 0
	const list: any[] = data?.official_items || []
	if (list.length > 0) {
		officialBillingMode.value = list[0].billing_mode || 'token'
		for (const item of list) {
			officialItems.push({
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
		const perSecond: Record<string, number> = list[0].per_second_prices || {}
		for (const [spec, price] of Object.entries(perSecond)) {
			officialPerSecondRows.push({ spec, price: Number(price) || 0 })
		}
	} else {
		// 无官方价：补一行空 token 行，保证官方编辑器模板绑定 officialItems[0] 安全
		ensureOfficialFirstRow()
	}
	const segConv = timeSegmentRowsFromAPI(data?.official_time_segments)
	officialTimeSegments.length = 0
	officialTimeSegments.push(...segConv.rows)
	officialTimeEnabled.value = segConv.enabled
	const ruleRows = paramRuleRowsFromAPI(data?.official_param_multipliers)
	officialParamRules.length = 0
	officialParamRules.push(...ruleRows)
}

function resetOfficialPricing() {
	officialBillingMode.value = 'token'
	officialItems.length = 0
	officialPerSecondRows.length = 0
	officialTimeSegments.length = 0
	officialTimeEnabled.value = false
	officialParamRules.length = 0
	ensureOfficialFirstRow()
}

// 保存载荷：官方定价 → official_items / official_time_segments / official_param_multipliers
// 三个数组恒传（后端 OfficialItems nil 才是「不动」），与主定价同为全量替换语义
function officialItemsPayload(): any[] {
	return officialItems.map((item, index) => ({
		billing_mode: officialBillingMode.value,
		min_tokens: officialBillingMode.value === 'tiered' ? item.min_tokens : 0,
		max_tokens: officialBillingMode.value === 'tiered' ? (index < officialItems.length - 1 ? item.max_tokens : null) : null,
		input_price: item.input_price,
		output_price: item.output_price,
		per_request_price: officialBillingMode.value === 'per_request' ? (item.per_request_price ?? null) : null,
		cache_read_price: item.cache_read_price,
		cache_creation_price: item.cache_creation_price,
		per_second_prices: officialBillingMode.value === 'per_second' ? officialPerSecondPayload() : null,
	}))
}

function officialPerSecondPayload(): Record<string, number> {
	const prices: Record<string, number> = {}
	for (const row of officialPerSecondRows) {
		const spec = row.spec.trim()
		if (spec) prices[spec] = Number(row.price) || 0
	}
	return prices
}

// 官方时段载荷：从行数组纯函数构建（不依赖编辑器挂载）；null=校验失败，调用方中止保存
function officialTimeSegmentsPayload(): any[] | null {
	const result = timeSegmentPayloadFrom(officialTimeSegments, officialTimeEnabled.value)
	if (result.error) {
		Message.warning(result.error)
		return null
	}
	return result.payload ?? []
}

// 官方参数倍率载荷：同官方时段，null=校验失败
function officialParamRulesPayload(): any[] | null {
	const result = paramRulePayloadFrom(officialParamRules)
	if (result.error) {
		Message.warning(result.error)
		return null
	}
	return result.payload ?? []
}

// 数据源源价 → 官方定价编辑器（100% 填入，USD 折算本位币 6 位小数；数据源只有平面价，
// 填入后仅改价格初值，人工核对修正并保存才落库）
function applySourceToOfficial(pricing: any) {
	if (!pricing) return
	if (officialBillingMode.value === 'per_second') {
		Message.warning('官方定价当前为按秒模式，数据源无按秒价格，请手动填写')
		return
	}
	if (officialBillingMode.value === 'tiered') {
		Message.warning('官方定价当前为阶梯模式，数据源为平面价，请手动调整各档位')
		return
	}
	if (officialItems.length === 0) {
		officialItems.push({ billing_mode: officialBillingMode.value, min_tokens: 0, max_tokens: null, input_price: 0, output_price: 0, per_request_price: null, cache_read_price: 0, cache_creation_price: 0, per_second_prices: null })
	}
	const first = officialItems[0]
	if (pricing.billing_mode === 'per_request') {
		officialBillingMode.value = 'per_request'
		first.per_request_price = toBasePrice(pricing.output_price)
	} else {
		if (officialBillingMode.value === 'per_request') {
			officialBillingMode.value = 'token'
		}
		first.input_price = toBasePrice(pricing.input_price)
		first.output_price = toBasePrice(pricing.output_price)
		first.cache_read_price = toBasePrice(pricing.cache_read_price)
		first.cache_creation_price = toBasePrice(pricing.cache_creation_price)
	}
	Message.success('已采用该数据源价格作为官方定价，请人工核对修正后保存')
}

// 派生比较：数据源价（USD）折本位币后与当前基准完全一致 → 该源即当前基准的来源
// （official_pricing 落库不存来源字段，只能前端比对推导；仅平面价模式可比）
function isCurrentBaseline(src: any): boolean {
	const p = src?.pricing
	if (!src?.found || !p) return false
	const first = officialFirst()
	if (p.billing_mode === 'per_request') {
		return (
			officialBillingMode.value === 'per_request' &&
			(Number(first.per_request_price) || 0) > 0 &&
			round6(toBasePrice(p.output_price)) === round6(Number(first.per_request_price) || 0)
		)
	}
	if (officialBillingMode.value !== 'token') return false
	return (
		round6(toBasePrice(p.input_price)) === round6(Number(first.input_price) || 0) &&
		round6(toBasePrice(p.output_price)) === round6(Number(first.output_price) || 0) &&
		round6(toBasePrice(p.cache_read_price)) === round6(Number(first.cache_read_price) || 0) &&
		round6(toBasePrice(p.cache_creation_price)) === round6(Number(first.cache_creation_price) || 0)
	)
}

// 按折扣换算（百分比）：官方定价 × 折扣 → 整体替换我方定价（价格维度 ×折扣，
// 时段与参数倍率原样复制——乘数是相对结构，不随折扣缩放）。纯前端计算，保留 6 位小数
const officialDiscountPercent = ref(100)

// 折扣显示文案：88.5% → "8.85 折"、50% → "5 折"
function foldLabel(percent: number | null | undefined): string {
	const p = Number(percent) || 0
	return String(+(p / 10).toFixed(2))
}

interface OfficialApplyRow {
	label: string
	official: number
	applied: number
	current: number
}

interface OfficialApplyPlan {
	ok: boolean
	reason: string // 不满足时的中文原因（预览区展示 + 按钮 disabled）
	hint: string // 可换算时的作用范围提示
	rows: OfficialApplyRow[]
	timeSegmentCount: number // 将原样复制的时段条数
	paramRuleCount: number // 将原样复制的参数倍率条数
	target: 'token' | 'per_request' | 'tiered' | 'per_second' // 将写入的我方计费模式
	tiers: Array<{ min_tokens: number; max_tokens: number | null; input_price: number; output_price: number; cache_read_price: number; cache_creation_price: number }> // tiered 换算结果
}

// 换算计划：预览表与执行动作共用同一套判断与计算，避免两份逻辑漂移
function officialApplyPlan(): OfficialApplyPlan {
	const plan: OfficialApplyPlan = { ok: false, reason: '', hint: '', rows: [], timeSegmentCount: 0, paramRuleCount: 0, target: 'token', tiers: [] }
	if (!hasOfficialPricing.value) {
		plan.reason = '官方定价尚未配置，请先从数据源采用或手动填写'
		return plan
	}
	const percent = Number(officialDiscountPercent.value)
	if (!percent || percent <= 0) {
		plan.reason = '请填写有效的折扣'
		return plan
	}
	const d = percent / 100
	const mode = officialBillingMode.value
	plan.target = mode as OfficialApplyPlan['target']
	if (editorBillingMode.value === 'per_second' && mode !== 'per_second') {
		plan.reason = '当前计费为按秒模式，官方定价需同为按秒才能换算填入'
		return plan
	}
	const first: any = editorItems[0] || {}
	const offFirst: any = officialFirst()
	if (mode === 'per_second') {
		// 官方按秒矩阵 × 折扣 → 整体替换我方矩阵（规格随官方定价）
		for (const r of officialPerSecondRows) {
			const spec = r.spec.trim()
			if (!spec) continue
			const official = Number(r.price) || 0
			plan.rows.push({
				label: spec,
				official,
				applied: round6(official * d),
				current: perSecondRows.find((c) => c.spec.trim() === spec)?.price || 0,
			})
		}
		if (!plan.rows.some((r) => r.applied > 0)) {
			plan.reason = '官方按秒定价矩阵中没有正价规格'
			return plan
		}
		plan.hint = '将整体替换我方按秒规格矩阵（规格随官方定价）'
	} else if (mode === 'per_request') {
		const official = Number(offFirst.per_request_price) || 0
		plan.rows.push({
			label: '按次单价',
			official,
			applied: round6(official * d),
			current: Number(first.per_request_price) || 0,
		})
		if (plan.rows[0].applied <= 0) {
			plan.reason = '官方按次定价尚未配置单价'
			return plan
		}
		plan.hint = '将整体替换我方按次单价'
	} else if (mode === 'tiered') {
		// 阶梯：逐档 input/output/缓存价 ×折扣，整体重建我方阶梯
		for (const [i, t] of officialItems.entries()) {
			const officialIn = Number(t.input_price) || 0
			const officialOut = Number(t.output_price) || 0
			const officialCacheRead = Number(t.cache_read_price) || 0
			const officialCacheCreation = Number(t.cache_creation_price) || 0
			plan.rows.push({
				label: `档位 ${i + 1}（≥${Number(t.min_tokens) || 0} tokens）`,
				official: officialIn,
				applied: round6(officialIn * d),
				current: Number(editorItems[i]?.input_price) || 0,
			})
			plan.rows.push({
				label: `档位 ${i + 1} 输出`,
				official: officialOut,
				applied: round6(officialOut * d),
				current: Number(editorItems[i]?.output_price) || 0,
			})
			if (officialCacheRead > 0) {
				plan.rows.push({
					label: `档位 ${i + 1} 缓存读取`,
					official: officialCacheRead,
					applied: round6(officialCacheRead * d),
					current: Number(editorItems[i]?.cache_read_price) || 0,
				})
			}
			if (officialCacheCreation > 0) {
				plan.rows.push({
					label: `档位 ${i + 1} 缓存创建`,
					official: officialCacheCreation,
					applied: round6(officialCacheCreation * d),
					current: Number(editorItems[i]?.cache_creation_price) || 0,
				})
			}
			plan.tiers.push({
				min_tokens: Number(t.min_tokens) || 0,
				max_tokens: i < officialItems.length - 1 ? (t.max_tokens ?? null) : null,
				input_price: round6(officialIn * d),
				output_price: round6(officialOut * d),
				cache_read_price: round6(officialCacheRead * d),
				cache_creation_price: round6(officialCacheCreation * d),
			})
		}
		if (!plan.tiers.some((t) => t.input_price > 0 || t.output_price > 0)) {
			plan.reason = '官方阶梯定价中没有任何正价档位'
			return plan
		}
		plan.hint = '将按官方档位整体重建我方阶梯定价（逐档缓存价 ×折扣）'
	} else {
		plan.target = 'token'
		const dims: Array<[string, number, number]> = [
			['输入价格', Number(offFirst.input_price) || 0, Number(first.input_price) || 0],
			['输出价格', Number(offFirst.output_price) || 0, Number(first.output_price) || 0],
			['缓存读取', Number(offFirst.cache_read_price) || 0, Number(first.cache_read_price) || 0],
			['缓存创建', Number(offFirst.cache_creation_price) || 0, Number(first.cache_creation_price) || 0],
		]
		for (const [label, official, current] of dims) {
			plan.rows.push({ label, official, applied: round6(official * d), current })
		}
		if (!plan.rows.some((r) => r.applied > 0)) {
			plan.reason = '官方定价尚未配置价格'
			return plan
		}
		plan.hint = '将覆盖我方按量四项价格，保留 6 位小数'
	}
	// 时段与参数倍率原样复制（乘数是相对结构，不随折扣缩放）
	plan.timeSegmentCount = officialTimeEnabled.value ? officialTimeSegments.length : 0
	plan.paramRuleCount = officialParamRules.length
	if (plan.timeSegmentCount > 0 || plan.paramRuleCount > 0) {
		plan.hint += `；另将原样复制官方的 ${plan.timeSegmentCount} 条时段定价与 ${plan.paramRuleCount} 条参数倍率规则`
	}
	plan.ok = true
	return plan
}

function applyOfficialToPricing() {
	const plan = officialApplyPlan()
	if (!plan.ok) {
		Message.warning(plan.reason)
		return
	}
	if (plan.target === 'per_second') {
		editorBillingMode.value = 'per_second'
		perSecondRows.length = 0
		perSecondRows.push(...plan.rows.map((r) => ({ spec: r.label, price: r.applied })))
	} else if (plan.target === 'per_request') {
		editorBillingMode.value = 'per_request'
		if (editorItems.length === 0) {
			editorItems.push(createEmptyItem('per_request'))
		}
		editorItems.splice(1)
		editorItems[0].per_request_price = plan.rows[0].applied
	} else if (plan.target === 'tiered') {
		editorBillingMode.value = 'tiered'
		editorItems.length = 0
		for (const t of plan.tiers) {
			editorItems.push({
				billing_mode: 'tiered',
				min_tokens: t.min_tokens,
				max_tokens: t.max_tokens,
				input_price: t.input_price,
				output_price: t.output_price,
				per_request_price: null,
				cache_read_price: t.cache_read_price,
				cache_creation_price: t.cache_creation_price,
				per_second_prices: null,
			})
		}
	} else {
		if (editorBillingMode.value === 'per_request') {
			editorBillingMode.value = 'token'
		}
		if (editorItems.length === 0) {
			editorItems.push(createEmptyItem('token'))
		}
		editorItems.splice(1)
		editorItems[0].input_price = plan.rows[0].applied
		editorItems[0].output_price = plan.rows[1].applied
		editorItems[0].cache_read_price = plan.rows[2].applied
		editorItems[0].cache_creation_price = plan.rows[3].applied
	}
	// 时段与参数倍率整体复制（直接克隆行数据，不经载荷往返；乘数是相对结构，不随折扣缩放）
	if (plan.timeSegmentCount > 0) {
		editorTimeSegments.length = 0
		editorTimeSegments.push(
			...officialTimeSegments.map((s) => ({
				...s,
				days: [...s.days],
				validRange: s.validRange ? [...s.validRange] : undefined,
			})),
		)
		timePricingEnabled.value = true
	}
	if (plan.paramRuleCount > 0) {
		paramRules.length = 0
		paramRules.push(...officialParamRules.map((r) => ({ ...r, conditions: r.conditions.map((c) => ({ ...c })) })))
	}
	Message.success(`已按官方价 ${foldLabel(Number(officialDiscountPercent.value))} 折填入（整体替换我方定价）`)
}

// 现价相对库内官方价的折扣摘要（营销口径自动计算，替代手算几折）。
// 官方与计费模式需同维可比（token↔token/tiered 取输入输出、按次↔按次、按秒↔按秒取基准价）
function foldText(officialVal: number, currentVal: number): string | null {
	if (officialVal <= 0 || currentVal <= 0) return null
	return String(+((currentVal / officialVal) * 10).toFixed(2))
}

function officialPerSecondBasePrice(): number {
	const prices = officialPerSecondRows.map((r) => Number(r.price) || 0).filter((p) => p > 0)
	if (prices.length === 0) return 0
	const wildcard = officialPerSecondRows.find((r) => r.spec.trim() === '*')
	return wildcard && wildcard.price > 0 ? wildcard.price : Math.min(...prices)
}

function officialFoldSummary(): string | null {
	const first: any = editorItems[0] || {}
	const offFirst: any = officialFirst()
	const parts: string[] = []
	if (officialBillingMode.value === 'per_second' && editorBillingMode.value === 'per_second') {
		const fold = foldText(officialPerSecondBasePrice(), perSecondBasePrice())
		if (fold) parts.push(`单价 ${fold} 折`)
	} else if (officialBillingMode.value === 'per_request' && editorBillingMode.value === 'per_request') {
		const fold = foldText(Number(offFirst.per_request_price) || 0, Number(first.per_request_price) || 0)
		if (fold) parts.push(`单价 ${fold} 折`)
	} else if (officialBillingMode.value === 'tiered' && editorBillingMode.value === 'tiered') {
		// 阶梯对阶梯：取首档（min_tokens=0）输入价比较
		const offEntry = officialItems.find((t: any) => (Number(t.min_tokens) || 0) === 0) || officialItems[0] || {}
		const fold = foldText(Number(offEntry.input_price) || 0, Number(first.input_price) || 0)
		if (fold) parts.push(`首档 ${fold} 折`)
	} else if (officialBillingMode.value === 'token' && (editorBillingMode.value === 'token' || editorBillingMode.value === 'tiered')) {
		const fi = foldText(Number(offFirst.input_price) || 0, Number(first.input_price) || 0)
		const fo = foldText(Number(offFirst.output_price) || 0, Number(first.output_price) || 0)
		if (fi) parts.push(`输入 ${fi} 折`)
		if (fo) parts.push(`输出 ${fo} 折`)
	}
	return parts.length ? parts.join(' · ') : null
}

// ============================================================
// Param Multipliers（参数倍率：行数据父级持有，编辑/校验/载荷在 PricingParamRulesEditor 组件）
// ============================================================
const paramRules = reactive<ParamRuleRow[]>([])
const paramEditorRef = ref<InstanceType<typeof PricingParamRulesEditor>>()

// ============================================================
// Load Pricing Data
// ============================================================
async function loadPricing() {
	if (!props.modelId) return
	// 模型切换时先清除上一模型的方案分发状态
	panelMode.value = 'generic'
	schemeConfigData.value = null
	editorItems.length = 0
	editorBillingMode.value = 'token'
	paramRules.length = 0
	resetOfficialData()
	editorLoading.value = true

	try {
		const res: any = await request.get(`/admin/models/${props.modelId}/pricing`)
		// 面板初始模式跟随库内 scheme（已配置、前端已登记且该模型名在方案白名单内 → 直达专属表单）
		const scheme = res.data?.data?.scheme
		schemeConfigData.value = res.data?.data?.scheme_config ?? null
		if (scheme && schemeOptions.value.some((o) => o.value === scheme)) {
			panelMode.value = scheme
		}
		// 通用表单数据照常加载（供切回通用模式时使用）
		const list: any[] = res.data?.data?.list || []
		editorPriceNote.value = res.data?.data?.price_note || ''
		editorDiscountLabel.value = res.data?.data?.discount_label || ''
		editorPriceChangeNote.value = res.data?.data?.price_change_note || ''
		if (list.length > 0) {
			// special 映射为 per_second：通用表单按等价的按秒矩阵形态编辑
			// （special 不进通用模式下拉；保存通用形态 = per_second + 清除 scheme，配对约束放行）
			const editorMode = list[0].billing_mode === 'special' ? 'per_second' : (list[0].billing_mode || 'token')
			editorBillingMode.value = editorMode
			editorItems.length = 0
			for (const item of list) {
				editorItems.push({
					billing_mode: item.billing_mode === 'special' ? 'per_second' : item.billing_mode,
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
		timeSegEditorRef.value?.loadTimeSegments(res.data?.data?.time_segments)
		paramEditorRef.value?.loadParamMultipliers(res.data?.data?.param_multipliers)
		loadOfficialPricing(res.data?.data)
	} catch {
		resetEditorDefaults()
	} finally {
		editorLoading.value = false
	}
}

// ============================================================
// Time Segment Pricing Editor（时段定价：行数据父级持有，编辑/校验/载荷在 PricingTimeSegmentsEditor 组件）
// ============================================================
const timePricingEnabled = ref(false)
const editorTimeSegments = reactive<TimeSegmentRow[]>([])
const timeSegEditorRef = ref<InstanceType<typeof PricingTimeSegmentsEditor>>()

// 组合预览基础价：随计费模式取当前编辑中的价格（tiered 用首档价）
const previewBase = computed<SegmentPreviewBase>(() => {
	const first: any = editorItems[0] || {}
	if (editorBillingMode.value === 'per_second') {
		return { kind: 'per_second', perSecond: perSecondBasePrice() }
	}
	if (editorBillingMode.value === 'per_request') {
		return { kind: 'per_request', perRequest: Number(first.per_request_price) || 0 }
	}
	const kind = editorBillingMode.value === 'tiered' ? 'tiered' : 'token'
	return { kind, input: Number(first.input_price) || 0, output: Number(first.output_price) || 0 }
})

// 官方时段组合预览基准价：随官方计费模式取官方编辑中的价格（与 previewBase 同构）
const officialPreviewBase = computed<SegmentPreviewBase>(() => {
	const first: any = officialFirst()
	if (officialBillingMode.value === 'per_second') {
		return { kind: 'per_second', perSecond: officialPerSecondBasePrice() }
	}
	if (officialBillingMode.value === 'per_request') {
		return { kind: 'per_request', perRequest: Number(first.per_request_price) || 0 }
	}
	const kind = officialBillingMode.value === 'tiered' ? 'tiered' : 'token'
	return { kind, input: Number(first.input_price) || 0, output: Number(first.output_price) || 0 }
})

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
		// 时段定价：编辑器组件校验并生成载荷（校验失败中止保存）
		const timeSegments = timeSegEditorRef.value?.buildTimeSegmentsPayload()
		if (timeSegments === null || timeSegments === undefined) {
			editorSaving.value = false
			return
		}
		// 参数倍率：编辑器组件校验并生成载荷（校验失败中止保存）
		const paramMultipliers = paramEditorRef.value?.buildParamMultipliersPayload()
		if (paramMultipliers === null || paramMultipliers === undefined) {
			editorSaving.value = false
			return
		}
		// 官方时段/参数倍率：行数组纯函数构建（编辑器未挂载也可用）；校验失败中止保存，与主定价一致
		const officialSegsPayload = officialTimeSegmentsPayload()
		if (officialSegsPayload === null) {
			editorSaving.value = false
			return
		}
		const officialRulesPayload = officialParamRulesPayload()
		if (officialRulesPayload === null) {
			editorSaving.value = false
			return
		}
		// 官方按秒矩阵：重复规格/负价校验（官方价为可选参照，空矩阵=未配置交后端清除，不强制正价）
		if (officialBillingMode.value === 'per_second') {
			const officialPerSecond = buildPerSecondMap(officialPerSecondRows, false)
			if (officialPerSecond.map === null) {
				if (officialPerSecond.error) Message.warning(officialPerSecond.error)
				editorSaving.value = false
				return
			}
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
			// 通用模式保存 = 全量替换语义：显式清除特殊计费方案声明（scheme 空串回落通用引擎并清 scheme_config）
			scheme: '',
			time_segments: timeSegments,
			param_multipliers: paramMultipliers,
			// 官方定价：三数组恒传（后端 OfficialItems nil 才是「不动」），全量替换语义；上方已校验
			official_items: officialItemsPayload(),
			official_time_segments: officialSegsPayload,
			official_param_multipliers: officialRulesPayload,
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

// 保存统一入口：特殊计费模式交给方案编辑器（自治校验与保存，失败自行提示），
// 成功提示与关弹窗由本外壳统一处理；通用模式走通用表单保存（全量替换语义清除 scheme）
async function onSave() {
	if (activeSchemeEditor.value) {
		editorSaving.value = true
		try {
			const ok = await schemeEditorRef.value?.save()
			if (ok) {
				Message.success('定价已保存')
				emit('update:visible', false)
				emit('saved')
			}
		} catch {
			// 方案编辑器内部已提示，此处不重复弹错
		} finally {
			editorSaving.value = false
		}
		return
	}
	if (panelMode.value !== 'generic' && !activeSchemeEditor.value) {
		Message.warning('该特殊计费方案暂无前端编辑器支持，请联系管理员')
		return
	}
	await savePricing()
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
		panelMode.value = 'generic'
		schemeConfigData.value = null
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
			<div class="pricing-footer">
				<!-- 计费模式下拉（左下角）：仅模型名命中方案白名单时显示，其余模型只有通用计费 -->
				<ASelect
					v-if="schemeOptions.length > 0"
					v-model="panelMode"
					size="small"
					class="pricing-mode-select"
					:style="{ width: '150px', flexShrink: 0 }"
				>
					<AOption value="generic">通用计费</AOption>
					<AOption v-for="opt in schemeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</AOption>
				</ASelect>
				<span v-else class="pricing-footer-placeholder" />
				<div class="pricing-footer-actions">
					<AButton @click="emit('update:visible', false)">取消</AButton>
					<AButton type="primary" :loading="editorSaving" @click="onSave">保存定价</AButton>
				</div>
			</div>
		</template>
		<ASpin :loading="editorLoading" style="width: 100%">
		<!-- 特殊计费方案：内容区整体交给方案专属编辑器（契约见 pricingSchemeEditors.ts） -->
		<component
			:is="activeSchemeEditor"
			v-if="activeSchemeEditor"
			:key="panelMode"
			ref="schemeEditorRef"
			:model-id="props.modelId"
			:scheme-config="schemeConfigData"
		/>
		<AForm v-else :model="{}" layout="vertical">
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
						<div class="billing-mode-tools">
							<!-- 官方价状态：颜色区分是否配置；点击标签进二层弹窗设置。已配置时右侧直接给快速折扣录入 -->
							<div
								class="official-quick"
								:title="hasOfficialPricing ? `官方价基准已设置（非计费依据）${officialFoldSummary() ? '，' + officialFoldSummary() : ''}；点击标签可修改` : '官方价基准未设置；点击标签设置后可按折扣一键生成我方定价'"
							>
								<ATag :color="hasOfficialPricing ? 'green' : 'gray'" class="official-quick-tag" @click="openOfficialModal">
									{{ hasOfficialPricing ? '官方价已设置' : '官方价未设置' }}
								</ATag>
								<template v-if="hasOfficialPricing">
									<AInputNumber
										v-model="officialDiscountPercent"
										:min="1"
										:max="1000"
										:step="5"
										:precision="1"
										size="small"
										class="official-quick-input"
										title="官方价 × 折扣 → 填入我方定价"
									>
										<template #suffix>%</template>
									</AInputNumber>
									<AButton size="small" type="primary" @click="applyOfficialToPricing">按官方价填入</AButton>
								</template>
							</div>
						</div>
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
									:precision="6"
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
									:precision="6"
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
									:precision="6"
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
									:precision="6"
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
									:precision="6"
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
									:precision="6"
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
										:precision="6"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="输出价格">
									<AInputNumber
										v-model="tier.output_price"
										:min="0"
										:precision="6"
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
										:precision="6"
										class="w-full"
									>
										<template #suffix>{{ currencySymbol }}/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="缓存创建价格">
									<AInputNumber
										v-model="tier.cache_creation_price"
										:min="0"
										:precision="6"
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

				<!-- Time Segment Pricing（时段定价：峰谷/工作时段/促销）——编辑器组件与官方弹窗共用 -->
				<PricingTimeSegmentsEditor
					ref="timeSegEditorRef"
					v-model:enabled="timePricingEnabled"
					:segments="editorTimeSegments"
					:preview-base="previewBase"
				/>

				<!-- Param Multipliers（参数倍率：按请求参数匹配加/折扣）——编辑器组件与官方弹窗共用 -->
				<PricingParamRulesEditor ref="paramEditorRef" :rules="paramRules" />

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

	<!-- 官方定价基准：二层弹窗（数据源参考价 → 官方价编辑 → 折扣换算填入），与我方定价解耦。
	     误关无损失：官方价改动留在组件 reactive 里，随主弹窗「保存定价」落库才生效 -->
	<AModal
		:visible="officialModalVisible"
		:title="`官方定价基准 - ${modelIdStr}`"
		:width="'min(880px, 96vw)'"
		:body-style="{ maxHeight: '72vh', overflowY: 'auto' }"
		@cancel="officialModalVisible = false"
		@close="officialModalVisible = false"
	>
		<!-- 官方定价（基准，可人工核对修正；结构与主定价同构，支持全部四种计费模式）。
		     数据源参考价为浮窗（APopover 挂在标题行按钮上），不混入页面内容 -->
		<div class="official-modal-section">
			<div class="official-modal-section-head">
				<h4>官方定价（基准）</h4>
				<span class="official-modal-section-hint">单位随本位币（{{ currencySymbol }}）</span>
				<APopover
					trigger="click"
					position="bottom"
					:content-style="{ padding: '12px', width: 'min(680px, 90vw)' }"
					@popup-visible-change="onSourcesVisibleChange"
				>
					<AButton size="mini" class="official-sources-trigger">
						<template #icon><IconCloudDownload /></template>
						数据源参考价
					</AButton>
					<template #content>
						<div class="official-sources-panel">
							<div class="official-sources-panel-head">
								<span class="official-sources-panel-title">数据源参考价</span>
								<AButton size="mini" :loading="officialLoading" @click="fetchOfficialPricing">
									<template #icon><IconRefresh /></template>
									重新拉取
								</AButton>
							</div>
							<div v-if="officialLoading && !officialData" class="official-reference-state">
								<ASpin />
								<span class="official-loading-text">正在拉取 4 个数据源…</span>
							</div>
							<AAlert v-else-if="officialError" type="error" :title="officialError" />
							<div v-else-if="officialData" class="official-source-grid">
								<div v-for="src in officialData.sources" :key="src.source" class="official-source-card">
									<div class="official-source-header">
										<ATag size="small" :color="getOfficialSourceMeta(src.source).color">
											{{ getOfficialSourceMeta(src.source).label }}
										</ATag>
										<template v-if="src.found && src.pricing">
											<ATag v-if="src.provider" size="small" color="orangered">{{ src.provider }}</ATag>
											<ATag v-if="src.mode" size="small">{{ src.mode }}</ATag>
											<ATag v-if="isCurrentBaseline(src)" size="small" color="green">已是当前基准</ATag>
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
										<AButton
											type="primary"
											size="small"
											long
											:disabled="officialBillingMode === 'per_second'"
											:title="officialBillingMode === 'per_second' ? '官方定价当前为按秒模式，数据源无按秒价格' : ''"
											@click="applySourceToOfficial(src.pricing)"
										>采用为官方价</AButton>
									</template>
									<div v-else class="official-not-found" :class="{ 'official-not-found--error': src.error }">
										{{ src.error ? '数据源暂时不可用' : '未找到该模型' }}
									</div>
								</div>
							</div>
						</div>
					</template>
				</APopover>
			</div>
			<div class="official-mode-row">
				<ARadioGroup v-model="officialBillingMode" type="button" size="small">
					<ARadio v-for="opt in officialBillingModeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
				</ARadioGroup>
			</div>

			<!-- 四种模式分支：officialItems 保证至少一行（首行由 ensureOfficialFirstRow 兜底），
			     v-if 再防御一次空数组渲染 -->
			<template v-if="officialItems.length > 0">
			<!-- 官方按次定价 -->
			<template v-if="officialBillingMode === 'per_request'">
				<div class="grid grid-cols-2 gap-x-4">
					<AFormItem label="官方按次单价">
						<AInputNumber
							v-model="officialItems[0].per_request_price"
							:min="0"
							:precision="6"
							placeholder="官方每次调用价格"
							class="w-full"
						>
							<template #suffix>{{ currencySymbol }} / 次</template>
						</AInputNumber>
					</AFormItem>
				</div>
			</template>

			<!-- 官方按秒定价（数据源无按秒价，按官方公布价格手动填写） -->
			<template v-else-if="officialBillingMode === 'per_second'">
				<div class="per-second-card">
					<div v-if="officialPerSecondRows.length" class="per-second-col-head">
						<span>规格</span>
						<span>每秒单价</span>
						<span class="per-second-op-col"></span>
					</div>
					<div v-for="(row, index) in officialPerSecondRows" :key="index" class="per-second-row">
						<AInput
							v-model="row.spec"
							placeholder="如 720p / 1080p / *（兜底）"
							:max-length="32"
							class="per-second-spec"
						/>
						<AInputNumber
							v-model="row.price"
							:min="0"
							:precision="6"
							placeholder="0"
							class="per-second-price"
						>
							<template #suffix>{{ currencySymbol }} / 秒</template>
						</AInputNumber>
						<AButton
							size="mini"
							status="danger"
							title="删除该规格"
							@click="removeOfficialPerSecondRow(index)"
						>−</AButton>
					</div>
					<div v-if="!officialPerSecondRows.length" class="per-second-empty">
						数据源无按秒价格，请按官方公布价格手动填写
					</div>
				</div>
				<div class="flex items-center gap-2 mt-2">
					<AButton size="small" type="outline" @click="addOfficialPerSecondRow()">+ 添加规格</AButton>
					<AButton
						v-for="preset in perSecondSpecPresets"
						:key="preset"
						size="mini"
						@click="addOfficialPerSecondRow(preset)"
					>{{ preset }}</AButton>
				</div>
			</template>

			<!-- 官方阶梯定价（与我方阶梯同构：逐档 min/max/输入/输出/缓存价） -->
			<template v-else-if="officialBillingMode === 'tiered'">
				<div v-for="(tier, index) in officialItems" :key="index" class="tier-card">
					<div class="tier-header">
						<span class="tier-label">第 {{ index + 1 }} 档</span>
						<AButton
							size="mini"
							status="danger"
							:disabled="officialItems.length <= 1"
							@click="removeOfficialTier(index)"
						>删除</AButton>
					</div>
					<div class="grid grid-cols-2 md:grid-cols-4 gap-x-4">
						<AFormItem label="起始 Token">
							<AInputNumber v-model="tier.min_tokens" :min="0" :precision="0" class="w-full" />
						</AFormItem>
						<AFormItem :label="index === officialItems.length - 1 ? '结束（无上限）' : '结束 Token'">
							<AInputNumber v-if="index < officialItems.length - 1" v-model="tier.max_tokens" :min="0" :precision="0" class="w-full" />
							<AInput v-else value="∞" disabled />
						</AFormItem>
						<AFormItem label="输入价格">
							<AInputNumber v-model="tier.input_price" :min="0" :precision="6" placeholder="0" class="w-full">
								<template #suffix>{{ currencySymbol }} / 1M</template>
							</AInputNumber>
						</AFormItem>
						<AFormItem label="输出价格">
							<AInputNumber v-model="tier.output_price" :min="0" :precision="6" placeholder="0" class="w-full">
								<template #suffix>{{ currencySymbol }} / 1M</template>
							</AInputNumber>
						</AFormItem>
					</div>
					<div class="grid grid-cols-2 gap-x-4 mt-1">
						<AFormItem label="缓存读取价格">
							<AInputNumber v-model="tier.cache_read_price" :min="0" :precision="6" class="w-full">
								<template #suffix>{{ currencySymbol }}/1M</template>
							</AInputNumber>
						</AFormItem>
						<AFormItem label="缓存创建价格">
							<AInputNumber v-model="tier.cache_creation_price" :min="0" :precision="6" class="w-full">
								<template #suffix>{{ currencySymbol }}/1M</template>
							</AInputNumber>
						</AFormItem>
					</div>
				</div>
				<AButton type="dashed" long class="mt-1" @click="addOfficialTier">+ 添加档位</AButton>
			</template>

			<!-- 官方按量定价 -->
			<template v-else>
				<div class="grid grid-cols-2 md:grid-cols-4 gap-x-4">
					<AFormItem label="输入价格">
						<AInputNumber
							v-model="officialItems[0].input_price"
							:min="0"
							:precision="6"
							placeholder="0"
							class="w-full"
						>
							<template #suffix>{{ currencySymbol }} / 1M</template>
						</AInputNumber>
					</AFormItem>
					<AFormItem label="输出价格">
						<AInputNumber
							v-model="officialItems[0].output_price"
							:min="0"
							:precision="6"
							placeholder="0"
							class="w-full"
						>
							<template #suffix>{{ currencySymbol }} / 1M</template>
						</AInputNumber>
					</AFormItem>
					<AFormItem label="缓存读取">
						<AInputNumber
							v-model="officialItems[0].cache_read_price"
							:min="0"
							:precision="6"
							placeholder="0"
							class="w-full"
						>
							<template #suffix>{{ currencySymbol }} / 1M</template>
						</AInputNumber>
					</AFormItem>
					<AFormItem label="缓存创建">
						<AInputNumber
							v-model="officialItems[0].cache_creation_price"
							:min="0"
							:precision="6"
							placeholder="0"
							class="w-full"
						>
							<template #suffix>{{ currencySymbol }} / 1M</template>
						</AInputNumber>
					</AFormItem>
				</div>
			</template>
			</template>

			<!-- 官方的时段定价与参数倍率（与主定价一致：始终展示，不做折叠开关） -->
			<PricingTimeSegmentsEditor
				v-model:enabled="officialTimeEnabled"
				:segments="officialTimeSegments"
				:preview-base="officialPreviewBase"
				title="时段定价（基准）"
				hint="随折扣填入时原样复制到主定价（乘数不缩放）"
			/>
			<PricingParamRulesEditor
				:rules="officialParamRules"
				title="参数倍率（基准）"
				hint="随折扣填入时原样复制到主定价（乘数不缩放）"
			/>
		</div>

		<template #footer>
			<!-- 底部提示占用现有 footer 插槽：左下角说明文字 + 右侧完成按钮 -->
			<div class="official-modal-footer">
				<span class="official-modal-footer-hint">
					官方价不是计费依据，仅作折扣换算基准与「现价几折」展示；人工核对修正后随主弹窗「保存定价」一并落库。
				</span>
				<AButton type="primary" @click="officialModalVisible = false">完成</AButton>
			</div>
		</template>
	</AModal>
</template>

<style scoped>
/* 定价面板 footer：左下角计费模式下拉 + 右侧操作按钮（窄屏自动换行） */
.pricing-footer {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 12px;
	flex-wrap: wrap;
}

.pricing-mode-select {
	width: 150px;
	max-width: 150px;
}

.pricing-footer-placeholder {
	flex: 1;
}

.pricing-footer-actions {
	display: flex;
	gap: 8px;
}

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

/* 计费模式行右上角工具组：官方价状态 + 快速折扣录入 + 折扣重算 */
.billing-mode-tools {
	display: flex;
	align-items: center;
	gap: 16px;
	flex-wrap: wrap;
}

/* 官方价状态标签：颜色区分是否配置；可点击进二层弹窗设置 */
.official-quick {
	display: flex;
	align-items: center;
	gap: 8px;
	flex-wrap: wrap;
}

.official-quick-tag {
	cursor: pointer;
}

.official-quick-input {
	width: 88px;
}

/* 官方价二层弹窗 footer：左下角提示说明 + 右侧完成按钮（复用 footer 插槽，非新增插槽） */
.official-modal-footer {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
}

.official-modal-footer-hint {
	min-width: 0;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-align: left;
}

@media (max-width: 639px) {
	.official-modal-footer {
		flex-direction: column-reverse;
		align-items: stretch;
	}

	.official-modal-footer-hint {
		text-align: center;
	}
}

.official-modal-section {
	margin-bottom: 20px;
}

.official-modal-section-head {
	display: flex;
	align-items: center;
	gap: 10px;
	margin-bottom: 10px;
}

.official-modal-section-head h4 {
	margin: 0;
	font-size: 13px;
	font-weight: 600;
	color: var(--ta-text-primary);
}

.official-modal-section-hint {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

/* 官方价数据源参考价：浮窗触发按钮推到标题行右侧 */
.official-sources-trigger {
	margin-left: auto;
}

/* 数据源参考价浮窗面板（宽度经 APopover content-style 设定，内容随触发元素渲染） */
.official-sources-panel {
	max-height: 56vh;
	overflow-y: auto;
}

.official-sources-panel-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
	margin-bottom: 10px;
}

.official-sources-panel-title {
	font-size: 13px;
	font-weight: 600;
	color: var(--ta-text-primary);
}

.official-reference-state {
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 12px;
	min-height: 120px;
}

.official-loading-text {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

/* 官方价二层弹窗：模式切换行 */
.official-mode-row {
	margin-bottom: 12px;
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
	.official-source-grid {
		grid-template-columns: 1fr;
	}
}
</style>
