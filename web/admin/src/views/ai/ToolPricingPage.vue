<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { Message } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { hasPermission } from '@/utils/permission'
import { displayCurrency, currencySymbol } from '@/composables/useCurrency'

// ===== 数据契约（与 internal/logic/billing/tool_pricing.go 严格对齐）=====
// sys_options.tool_call_prices = JSON 对象：
//   key   = 工具名 或 工具名:模型前缀*（尾部 * 可省略，语义为纯前缀匹配；同名多条时最长前缀优先）
//   value = 每次调用单价（本位币，0 ~ 100）
// 这是唯一价源：程序不内置任何工具价格，表中未出现的工具/模型前缀一律不计费。
// 单价 0 与未配置的计费结果相同（都是免费），填 0 仅用于把「这个工具就该免费」的意图写进配置。
const TOOL_PATTERN = /^[a-z0-9_]+$/
const PREFIX_PATTERN = /^[A-Za-z0-9._*\-]+$/
const MAX_PRICE = 100

interface PriceRow {
	uid: number
	tool: string
	prefix: string
	price: number | undefined
}

const loading = ref(false)
const saving = ref(false)
const rows = ref<PriceRow[]>([])
let seq = 0

// 已保存快照（合法行序列化串），用于未保存修改检测与离开确认
let savedSnapshot = ''
const dirty = ref(false)

// 模型前缀自动补全候选：模型列表页同源下拉（含全部启用模型标识）
const modelIdOptions = ref<string[]>([])

function rowKey(r: PriceRow): string {
	const tool = r.tool.trim()
	const prefix = r.prefix.trim()
	if (!tool) return ''
	return prefix ? `${tool}:${prefix}` : tool
}

function rowError(r: PriceRow): string {
	const tool = r.tool.trim()
	const prefix = r.prefix.trim()
	if (!tool && !prefix) return ''
	if (!tool) return '缺少工具名'
	if (!TOOL_PATTERN.test(tool)) return '工具名只能是小写字母、数字、下划线'
	if (prefix && !PREFIX_PATTERN.test(prefix)) return '模型前缀只能含字母、数字、点、横线、下划线和 *'
	const p = Number(r.price)
	if (r.price === undefined || r.price === null || !Number.isFinite(p) || p < 0 || p > MAX_PRICE) {
		return `单价需在 0 ~ ${MAX_PRICE} 之间`
	}
	return ''
}

// 字段级错误态：错误文案归属哪个输入框（Input 的 error 会透传到内部 Input）
function toolInvalid(r: PriceRow): boolean {
	return rowError(r).includes('工具名')
}
function prefixInvalid(r: PriceRow): boolean {
	return rowError(r).includes('模型前缀')
}

// 合法且非空的行才参与保存，避免半填状态把脏 key 落到配置里
function validRows(): PriceRow[] {
	return rows.value.filter((r) => rowKey(r) !== '' && rowError(r) === '')
}

// 合法行 → 价格表对象；价格固定 6 位小数与后端存储精度对齐
function buildPrices(): Record<string, number> {
	const obj: Record<string, number> = {}
	for (const r of validRows()) {
		obj[rowKey(r)] = Number((r.price ?? 0).toFixed(6))
	}
	return obj
}

// 任意编辑后标记未保存；保存/加载成功后重置
function markDirty() {
	dirty.value = true
}

function parseRows(prices: Record<string, unknown>) {
	rows.value = Object.entries(prices).map(([key, price]) => {
		const at = key.indexOf(':')
		return {
			uid: ++seq,
			tool: at < 0 ? key : key.slice(0, at),
			prefix: at < 0 ? '' : key.slice(at + 1),
			price: Number(price),
		}
	})
	dirty.value = false
	savedSnapshot = JSON.stringify(buildPrices())
}

function addRow() {
	rows.value.unshift({ uid: ++seq, tool: '', prefix: '', price: undefined })
	dirty.value = true
}

function removeRow(uid: number) {
	rows.value = rows.value.filter((r) => r.uid !== uid)
	dirty.value = true
}

// ===== 加载 / 保存 =====
async function load() {
	loading.value = true
	try {
		const res: any = await request.get('/admin/models/tool-pricing')
		const prices = res.data?.data?.prices
		parseRows(prices && typeof prices === 'object' ? prices : {})
	} catch {
		// 错误提示由拦截器统一处理
	} finally {
		loading.value = false
	}
}

// 前缀补全候选：与模型列表页同源（不额外拉接口则复用 load 内失败静默）
async function loadModelOptions() {
	try {
		const res: any = await request.get('/admin/models/options')
		const list = res.data?.data?.list
		if (Array.isArray(list)) {
			modelIdOptions.value = list.map((m: any) => m.model_id).filter(Boolean)
		}
	} catch {
		// 候选拉不到只影响补全体验，不阻断页面
	}
}

function invalidCount(): number {
	return rows.value.filter((r) => rowError(r) !== '').length
}

async function save() {
	const invalid = invalidCount()
	if (invalid > 0) {
		Message.warning(`有 ${invalid} 行存在校验错误，保存时这些行将被忽略`)
	}
	saving.value = true
	try {
		await request.put('/admin/models/tool-pricing', { prices: buildPrices() })
		savedSnapshot = JSON.stringify(buildPrices())
		dirty.value = false
		Message.success('工具价格已保存')
	} catch {
		// 错误提示由拦截器统一处理
	} finally {
		saving.value = false
	}
}

onMounted(() => {
	load()
	loadModelOptions()
})

// 有未保存修改时离开页面前确认（快照兜底：编辑后还原为已保存内容不算脏）
onBeforeRouteLeave(() => {
	if (dirty.value && JSON.stringify(buildPrices()) !== savedSnapshot) {
		return window.confirm('有未保存的工具价格修改，确定离开？')
	}
	return true
})

// ===== 派生状态：统计 / 重复 =====
const duplicateKeys = computed(() => {
	const counts = new Map<string, number>()
	for (const r of rows.value) {
		const k = rowKey(r)
		if (!k) continue
		counts.set(k, (counts.get(k) ?? 0) + 1)
	}
	return new Set([...counts.entries()].filter(([, n]) => n > 1).map(([k]) => k))
})

// 已配置项按 key 索引（仅合法行），用于「配 0 = 标注免费」的判定
const configuredByKey = computed(() => {
	const map = new Map<string, number>()
	for (const r of rows.value) {
		const k = rowKey(r)
		if (!k || rowError(r) !== '') continue
		map.set(k, Number(r.price ?? 0))
	}
	return map
})

const validCount = computed(() => validRows().length)
const freeCount = computed(() => validRows().filter((r) => Number(r.price ?? 0) === 0).length)
const errorCount = computed(() => invalidCount())

// 模型前缀档数量：仅用于概览展示，不参与计费
const prefixRowCount = computed(() => validRows().filter((r) => r.prefix.trim() !== '').length)

const canEdit = hasPermission('model:edit')
</script>

<template>
	<div class="page-table">
		<PageHeader
			title="工具调用价格"
			description="云端工具 / 联网搜索按调用次数单独计费，与 token 费用分开结算，不参与租户折扣、时段与参数倍率。未配置的工具不计费（免费）。"
		>
			<template #actions>
				<AButton v-if="canEdit" size="small" @click="addRow">新增价格项</AButton>
				<AButton
					v-if="canEdit"
					type="primary"
					:loading="saving"
					:disabled="!dirty"
					@click="save"
				>保存{{ dirty ? '' : '（无改动）' }}</AButton>
			</template>
		</PageHeader>

		<!-- 概览：数量与口径 -->
		<div class="tp-overview">
			<div class="tp-overview__stats">
				<div class="tp-overview__stat">
					<span class="tp-overview__num">{{ validCount }}</span>
					<span class="tp-overview__label">价格项</span>
				</div>
				<div class="tp-overview__stat">
					<span class="tp-overview__num">{{ prefixRowCount }}</span>
					<span class="tp-overview__label">模型前缀档</span>
				</div>
				<div class="tp-overview__stat">
					<span class="tp-overview__num">{{ freeCount }}</span>
					<span class="tp-overview__label">免费项</span>
				</div>
				<div v-if="errorCount" class="tp-overview__stat tp-overview__stat--danger">
					<span class="tp-overview__num">{{ errorCount }}</span>
					<span class="tp-overview__label">待修正</span>
				</div>
			</div>
			<div class="tp-overview__currency">
				本位币 <b>{{ displayCurrency }}</b>
				<span class="tp-overview__hint">价格直接按本位币填写，不随汇率变动</span>
			</div>
		</div>

		<!-- 价格项列表 -->
		<div v-if="rows.length === 0" class="tp-empty">
			<div class="tp-empty__title">{{ loading ? '正在加载价格表…' : '还没有配置任何工具价格' }}</div>
			<div v-if="!loading" class="tp-empty__desc">
				当前所有工具调用都不产生附加费。点右上角「新增价格项」，为需要收费的工具填写单价；
				需要按模型区分价格时，在冒号后填模型前缀（如 <code>gpt-4o*</code>）。
			</div>
		</div>
		<div v-else class="tp-rows" :class="{ 'tp-rows--loading': loading }">
			<div class="tp-row tp-row--head">
				<span>工具名 / 模型前缀</span>
				<span>单价（{{ currencySymbol }} / 次）</span>
				<span></span>
			</div>
			<div
				v-for="row in rows"
				:key="row.uid"
				class="tp-row"
				:class="{ 'tp-row--invalid': rowError(row) !== '' }"
			>
				<div class="tp-row__key">
					<AInput
						v-model="row.tool"
						class="tp-row__tool"
						placeholder="如 web_search"
						:error="toolInvalid(row)"
						:disabled="!canEdit"
						@change="markDirty"
					/>
					<span class="tp-row__colon">:</span>
					<AAutoComplete
						v-model="row.prefix"
						class="tp-row__prefix"
						:data="modelIdOptions"
						allow-clear
						placeholder="模型前缀，如 gpt-4o*（留空 = 全部模型）"
						:error="prefixInvalid(row)"
						:disabled="!canEdit"
						@change="markDirty"
					/>
				</div>
				<div class="tp-row__price">
					<AInputNumber
						v-model="row.price"
						:min="0"
						:max="MAX_PRICE"
						:step="0.001"
						placeholder="0.01"
						:disabled="!canEdit"
						@change="markDirty"
					/>
				</div>
				<div class="tp-row__ops">
					<ATag v-if="duplicateKeys.has(rowKey(row))" size="small" color="orange">重复</ATag>
					<ATag
						v-else-if="configuredByKey.has(rowKey(row)) && configuredByKey.get(rowKey(row)) === 0"
						size="small"
					>免费</ATag>
					<AButton
						v-if="canEdit"
						size="mini"
						type="text"
						status="danger"
						@click="removeRow(row.uid)"
					>删除</AButton>
				</div>
				<div v-if="rowError(row)" class="tp-row__error">{{ rowError(row) }}</div>
			</div>
		</div>

		<div class="tp-tip">
			匹配顺序：<b>最长模型前缀</b> → 工具默认价 → 免费。前缀尾部 <code>*</code> 可省略（语义相同），
			例如 <code>web_search:gpt-4o*</code> 与 <code>web_search:gpt-4o</code> 等价。
		</div>
	</div>
</template>

<style scoped>
/* ===== 概览 ===== */
.tp-overview {
	display: flex;
	align-items: center;
	justify-content: space-between;
	flex-wrap: wrap;
	gap: 12px 20px;
	padding: 12px 16px;
	margin-bottom: 16px;
	border-radius: var(--ta-radius);
	background: var(--ta-bg-secondary);
}
.tp-overview__stats {
	display: flex;
	align-items: center;
	gap: 20px;
}
.tp-overview__stat {
	display: flex;
	align-items: baseline;
	gap: 6px;
}
.tp-overview__num {
	font-size: 20px;
	font-weight: 600;
	color: var(--ta-text-primary);
	font-variant-numeric: tabular-nums;
	line-height: 1;
}
.tp-overview__stat--danger .tp-overview__num {
	color: var(--ta-danger);
}
.tp-overview__label {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}
.tp-overview__currency {
	font-size: 12px;
	color: var(--ta-text-secondary);
	text-align: right;
}
.tp-overview__currency b {
	font-weight: 600;
	color: var(--ta-text-primary);
}
.tp-overview__hint {
	display: block;
	margin-top: 2px;
	color: var(--ta-text-tertiary);
}

/* ===== 价格项列表 ===== */
.tp-rows {
	display: flex;
	flex-direction: column;
	border: 1px solid var(--ta-border);
	border-radius: var(--ta-radius);
	overflow: hidden;
}
.tp-rows--loading {
	opacity: 0.6;
}
.tp-row {
	display: grid;
	grid-template-columns: minmax(0, 1fr) 150px 128px;
	gap: 16px;
	align-items: center;
	padding: 10px 14px;
	position: relative;
}
.tp-row + .tp-row {
	border-top: 1px solid var(--color-border-2);
}
.tp-row--head {
	padding: 8px 14px;
	background: var(--ta-bg-secondary);
	font-size: 12px;
	color: var(--ta-text-tertiary);
}
.tp-row:not(.tp-row--head):hover {
	background: var(--ta-bg-secondary);
}
.tp-row--invalid {
	background: rgba(239, 68, 68, 0.03);
}
.tp-row--invalid::before {
	content: '';
	position: absolute;
	left: 0;
	top: 8px;
	bottom: 8px;
	width: 2px;
	border-radius: 2px;
	background: var(--ta-danger);
}
/* 工具名与模型前缀合并为一个键编辑区：工具名定宽、前缀吃剩余空间、冒号居间 */
.tp-row__key {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}
.tp-row__tool {
	flex: 0 0 180px;
	min-width: 0;
}
.tp-row__prefix {
	flex: 1;
	min-width: 0;
}
.tp-row__colon {
	color: var(--ta-text-tertiary);
	font-family: var(--font-family-mono, ui-monospace, monospace);
	flex-shrink: 0;
}
.tp-row__price {
	min-width: 0;
}
.tp-row__price :deep(.arco-input-number),
.tp-row__price :deep(.arco-auto-complete) {
	width: 100%;
	min-width: 0;
}
.tp-row__ops {
	display: flex;
	align-items: center;
	justify-content: flex-end;
	gap: 8px;
	min-width: 0;
}
.tp-row__error {
	grid-column: 1 / -1;
	margin-top: -2px;
	font-size: 12px;
	color: var(--ta-danger);
}

/* 窄屏：键编辑区独占一行，价格与操作并排 */
@media (max-width: 768px) {
	.tp-row {
		grid-template-columns: minmax(0, 1fr) 128px;
		row-gap: 8px;
	}
	.tp-row__key {
		grid-column: 1 / -1;
	}
	.tp-row__tool {
		flex: 0 0 140px;
	}
	.tp-row--head {
		display: none;
	}
}

/* ===== 空态 ===== */
.tp-empty {
	padding: 24px;
	border: 1px dashed var(--ta-border);
	border-radius: var(--ta-radius);
	background: var(--ta-bg-secondary);
	text-align: center;
}
.tp-empty__title {
	font-size: 13px;
	font-weight: 500;
	color: var(--ta-text-secondary);
}
.tp-empty__desc {
	margin-top: 4px;
	font-size: 12px;
	line-height: 1.7;
	color: var(--ta-text-tertiary);
}
.tp-empty__desc code {
	padding: 1px 4px;
	border-radius: 3px;
	background: var(--ta-bg-card);
	font-family: var(--font-family-mono, ui-monospace, monospace);
}

.tp-tip {
	margin-top: 12px;
	font-size: 12px;
	line-height: 1.7;
	color: var(--ta-text-tertiary);
}
.tp-tip code {
	padding: 1px 4px;
	border-radius: 3px;
	background: var(--ta-bg-secondary);
	font-family: var(--font-family-mono, ui-monospace, monospace);
}
</style>
