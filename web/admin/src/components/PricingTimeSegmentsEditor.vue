<script lang="ts">
// 行数据结构导出（<script setup> 不支持 export，类型声明放前置块）
export interface TimeSegmentRow {
	name: string
	dayPreset: string // every / weekday / weekend / custom
	days: number[] // 仅 custom 使用，1=周一..7=周日
	start_time: string // HH:mm，与 end_time 均空=全天
	end_time: string
	validRange: string[] | undefined // 有效期 [from, to]，可选（促销/定时调价）
	percent: number // 乘数百分比，50 = 半价（提交时换算为 multiplier）
}

// 组合预览基准价（父级按当前计费模式计算；不传则预览只显示倍率百分比）
export interface SegmentPreviewBase {
	kind: 'token' | 'tiered' | 'per_request' | 'per_second'
	input?: number
	output?: number
	perRequest?: number
	perSecond?: number
}

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

// API 时段列表 → 行编辑态。父级在编辑器组件未挂载时（如二层弹窗未打开/折叠区未展开）
// 也用本函数直接填充行数组，与组件内 loadTimeSegments 共用同一换算
export function timeSegmentRowsFromAPI(list: any[] | null | undefined): { rows: TimeSegmentRow[]; enabled: boolean } {
	const rows: TimeSegmentRow[] = []
	if (Array.isArray(list)) {
		for (const seg of list) {
			rows.push({
				name: seg.name || '',
				dayPreset: daysToPreset(seg.days),
				days: Array.isArray(seg.days) ? [...seg.days] : [],
				start_time: seg.start_time || '',
				end_time: seg.end_time || '',
				validRange: seg.valid_from || seg.valid_to ? [seg.valid_from || '', seg.valid_to || ''] : undefined,
				percent: Math.round((Number(seg.multiplier) || 1) * 10000) / 100,
			})
		}
	}
	return { rows, enabled: rows.length > 0 }
}

// 行编辑态 → 提交载荷（percent → multiplier、预设 → days，含前置校验）。
// error 非空=校验失败文案（提示方式由调用方决定，组件内走 Message.warning）
export function timeSegmentPayloadFrom(rows: TimeSegmentRow[], enabled: boolean): { payload?: any[]; error?: string } {
	if (!enabled) return { payload: [] }
	if (rows.length === 0) {
		return { error: '已开启时段定价，请至少添加一条时段' }
	}
	for (const [i, seg] of rows.entries()) {
		if (!seg.name.trim()) {
			return { error: `第 ${i + 1} 条时段名称不能为空` }
		}
		if (!seg.percent || seg.percent <= 0) {
			return { error: `时段「${seg.name}」乘数必须大于 0` }
		}
		if ((seg.start_time ? 1 : 0) !== (seg.end_time ? 1 : 0)) {
			return { error: `时段「${seg.name}」开始与结束时间需同时填写或同时留空` }
		}
		if (seg.dayPreset === 'custom' && seg.days.length === 0) {
			return { error: `时段「${seg.name}」自定义适用日至少选择一天` }
		}
	}
	return {
		payload: rows.map((seg) => ({
			name: seg.name.trim(),
			days: seg.dayPreset === 'custom' ? [...seg.days].sort((a, b) => a - b) : presetToDays[seg.dayPreset] || [],
			start_time: seg.start_time || '',
			end_time: seg.end_time || '',
			valid_from: seg.validRange?.[0] || '',
			valid_to: seg.validRange?.[1] || '',
			multiplier: (seg.percent || 0) / 100,
		})),
	}
}
</script>

<script setup lang="ts">
// 时段定价编辑器（主弹窗与官方定价弹窗共用）：
// 按命名时段对基准价等比缩放，时段从上到下先命中先生效。
// 数据数组由父级持有（reactive），编辑/校验/载荷逻辑内聚在本组件。
import { computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import { formatBilling } from '@/composables/useCurrency'

const props = withDefaults(
	defineProps<{
		segments: TimeSegmentRow[]
		previewBase?: SegmentPreviewBase | null
		title?: string
		hint?: string
	}>(),
	{
		previewBase: null,
		title: '时段定价',
		hint: '全部价格（含各档位/缓存/按次价）按乘数等比缩放：100% = 默认价，未命中时段按默认价',
	},
)

// 开关：父级可 v-model:enabled；关闭时段不参与提交
const enabled = defineModel<boolean>('enabled', { default: false })

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

// 预设/days 换算在前置块（timeSegmentRowsFromAPI / timeSegmentPayloadFrom 共用）

function createEmptySegment(): TimeSegmentRow {
	return { name: '', dayPreset: 'every', days: [], start_time: '', end_time: '', validRange: undefined, percent: 100 }
}

function addTimeSegment() {
	props.segments.push(createEmptySegment())
	if (!enabled.value) enabled.value = true
}

function removeTimeSegment(index: number) {
	props.segments.splice(index, 1)
}

// 上移/下移：时段从上到下先命中先生效，促销时段应排在常驻时段之前
function moveTimeSegment(index: number, offset: number) {
	const target = index + offset
	if (target < 0 || target >= props.segments.length) return
	const [seg] = props.segments.splice(index, 1)
	props.segments.splice(target, 0, seg)
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

function segmentPreview(seg: TimeSegmentRow): string {
	const m = (seg.percent || 0) / 100
	const base = props.previewBase
	if (!base) return `×${m}`
	if (base.kind === 'per_second') {
		return `${formatBilling((base.perSecond || 0) * m, 4)} /秒`
	}
	if (base.kind === 'per_request') {
		return `${formatBilling((base.perRequest || 0) * m, 4)} /次`
	}
	const prefix = base.kind === 'tiered' ? '首档 ' : ''
	return `${prefix}输入 ${formatBilling((base.input || 0) * m, 4)} · 输出 ${formatBilling((base.output || 0) * m, 4)}`
}

function defaultPreviewText(): string {
	const base = props.previewBase
	if (!base) return '—'
	if (base.kind === 'per_second') {
		return `${formatBilling(base.perSecond || 0, 4)} /秒`
	}
	if (base.kind === 'per_request') {
		return `${formatBilling(base.perRequest || 0, 4)} /次`
	}
	const prefix = base.kind === 'tiered' ? '首档 ' : ''
	return `${prefix}输入 ${formatBilling(base.input || 0, 4)} · 输出 ${formatBilling(base.output || 0, 4)}`
}

const hasSegments = computed(() => props.segments.length > 0)

// 回显：multiplier → percent，days → 预设（委托前置块纯函数，与父级直接填充行数组共用换算）
function loadTimeSegments(list: any[] | null | undefined) {
	const conv = timeSegmentRowsFromAPI(list)
	props.segments.length = 0
	props.segments.push(...conv.rows)
	enabled.value = conv.enabled
}

function resetTimeSegments() {
	enabled.value = false
	props.segments.length = 0
}

// 提交载荷：percent → multiplier，预设 → days（含前置校验，校验失败 Message 提示并返回 null）
function buildTimeSegmentsPayload(): any[] | null {
	const result = timeSegmentPayloadFrom(props.segments, enabled.value)
	if (result.error) {
		Message.warning(result.error)
		return null
	}
	return result.payload ?? []
}

defineExpose({ loadTimeSegments, resetTimeSegments, buildTimeSegmentsPayload })
</script>

<template>
	<div class="editor-section">
		<div class="editor-section-header">
			<h3>{{ title }}</h3>
			<ASwitch v-model="enabled" size="small" />
			<span class="section-hint">{{ hint }}</span>
		</div>
		<template v-if="enabled">
			<div v-for="(seg, index) in segments" :key="index" class="tier-card">
				<div class="tier-header">
					<div class="segment-tools">
						<AButton size="mini" :disabled="index === 0" @click="moveTimeSegment(index, -1)">↑</AButton>
						<AButton size="mini" :disabled="index === segments.length - 1" @click="moveTimeSegment(index, 1)">↓</AButton>
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
					<AFormItem v-if="seg.dayPreset === 'custom'" label="自定义星期" class="custom-days-item md:col-span-2">
						<div class="custom-days">
							<ACheckboxGroup v-model="seg.days" :options="weekdayOptions" />
						</div>
					</AFormItem>
				</div>
				<div class="segment-preview">{{ segmentDesc(seg) }} → {{ segmentPreview(seg) }}</div>
			</div>
			<AButton type="dashed" long class="mt-3" @click="addTimeSegment">+ 添加时段</AButton>
			<div class="combo-preview-hint">时段从上到下先命中先生效：限时促销时段应排在常驻时段上方</div>

			<!-- 组合预览：换算后的实际价格（父级未传基准价时不显示金额，仅倍率） -->
			<div v-if="hasSegments" class="combo-preview">
				<div class="combo-preview-title">价格预览（换算后实际价格）</div>
				<div class="combo-row">
					<span class="combo-name">默认价（未命中时段）</span>
					<span class="combo-value">{{ defaultPreviewText() }}</span>
				</div>
				<div v-for="(seg, index) in segments" :key="index" class="combo-row">
					<span class="combo-name">{{ seg.name || `时段 ${index + 1}` }}（{{ segmentDesc(seg) }}）</span>
					<span class="combo-value">{{ segmentPreview(seg) }}</span>
				</div>
			</div>
		</template>
	</div>
</template>

<style scoped>
/* 与主弹窗一致的 section / 卡片基础样式（scoped 隔离，组件内自持） */
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

.combo-value {
	color: var(--ta-text-primary);
	font-family: monospace;
	text-align: right;
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
}

.time-sep {
	color: var(--color-text-3);
}

.custom-days {
	display: flex;
	flex-wrap: wrap;
	gap: 4px 12px;
}

.segment-preview {
	margin-top: 4px;
	padding: 6px 10px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	background: var(--color-fill-2);
	border-radius: 4px;
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

.combo-preview-hint {
	margin-top: 6px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}
</style>
