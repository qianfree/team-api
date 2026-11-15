<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconDelete, IconEdit, IconPlus, IconDown, IconQuestionCircle } from '@arco-design/web-vue/es/icon'
import {
	type RequestRule,
	type RuleCondition,
	type ConditionKind,
	WEEKDAY_LABELS,
	PARAM_OP_LABELS,
	HEADER_OP_LABELS,
	RULE_TEMPLATES,
	newCondition,
	cloneRule,
	serializeRule,
	serializeRules,
	parseRules,
	validateRule,
	describeRule,
} from '@/utils/requestRules'

// 请求规则编辑器（表达式计费的 formula_rules）：规则卡片列表 + 弹窗编辑，
// 与表达式文本双向转换。表达式文本是唯一存储形态（v-model），可视化列表由其解析而来；
// 无法用可视化表示的手写表达式保留在表达式模式编辑，不做有损转换

const props = defineProps<{ modelValue: string }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const mode = ref<'visual' | 'raw'>('visual')
const rules = ref<RequestRule[]>([])
const rawText = ref(props.modelValue || '')
const parseError = ref('')
// 最近一次由本组件发出的文本：外部回写同值时不重新解析（避免打断编辑态）
let lastEmitted: string | null = null

function loadFromText(text: string) {
	rawText.value = text
	const res = parseRules(text)
	if (res.ok) {
		rules.value = res.rules
		parseError.value = ''
		mode.value = 'visual'
	} else {
		parseError.value = res.error
		mode.value = 'raw'
	}
}

watch(
	() => props.modelValue,
	(v) => {
		if (v === lastEmitted) return
		loadFromText(v || '')
	},
	{ immediate: true },
)

function emitText(text: string) {
	lastEmitted = text
	rawText.value = text
	emit('update:modelValue', text)
}

function commitRules() {
	emitText(serializeRules(rules.value))
}

function onRawInput(v: string) {
	emitText(v)
}

function switchToVisual() {
	const res = parseRules(rawText.value)
	if (!res.ok) {
		Message.warning(`无法转换为可视化规则：${res.error}`)
		parseError.value = res.error
		return
	}
	rules.value = res.rules
	parseError.value = ''
	mode.value = 'visual'
	// 规范化输出（统一括号/换行），语义不变
	commitRules()
}

function switchToRaw() {
	parseError.value = ''
	mode.value = 'raw'
}

function removeRule(idx: number) {
	rules.value.splice(idx, 1)
	commitRules()
}

function addTemplate(label: string | number | Record<string, any> | undefined) {
	const tpl = RULE_TEMPLATES.find((t) => t.label === label)
	if (!tpl) return
	rules.value.push(cloneRule(tpl.rule))
	commitRules()
	Message.success(`已添加规则：${tpl.label}`)
}

// 规则倍数的展示色：加价橙、折扣绿
function multiplierClass(m: number) {
	return m > 1 ? 'rr-mult--up' : 'rr-mult--down'
}

// ============================ 编辑弹窗 ============================

const editorVisible = ref(false)
const editingIndex = ref(-1) // -1 = 新增
const draft = ref<RequestRule>({ multiplier: 2, timezone: '', conditions: [] })

const kindOptions = [
	{ label: '星期', value: 'weekday' },
	{ label: '时间段', value: 'time' },
	{ label: '请求参数', value: 'param' },
	{ label: '请求头', value: 'header' },
]
const weekdayOptions = [1, 2, 3, 4, 5, 6, 0].map((d) => ({ label: WEEKDAY_LABELS[d], value: d }))
const paramOpOptions = Object.entries(PARAM_OP_LABELS).map(([value, label]) => ({ label, value }))
const headerOpOptions = Object.entries(HEADER_OP_LABELS).map(([value, label]) => ({ label, value }))
const timezoneOptions = ['Asia/Shanghai', 'UTC', 'Asia/Tokyo', 'Asia/Singapore', 'Europe/London', 'America/New_York', 'America/Los_Angeles']

const draftError = computed(() => validateRule(draft.value))
const draftPreview = computed(() => (draftError.value ? '' : serializeRule(draft.value)))
const draftHasTime = computed(() => draft.value.conditions.some((c) => c.kind === 'weekday' || c.kind === 'time'))

function openCreate() {
	editingIndex.value = -1
	draft.value = { multiplier: 2, timezone: '', conditions: [newCondition('param')] }
	editorVisible.value = true
}

function openEdit(idx: number) {
	editingIndex.value = idx
	draft.value = cloneRule(rules.value[idx])
	editorVisible.value = true
}

function addCondition(kind: string | number | Record<string, any> | undefined) {
	draft.value.conditions.push(newCondition(kind as ConditionKind))
}

function changeKind(idx: number, kind: ConditionKind) {
	draft.value.conditions.splice(idx, 1, newCondition(kind))
}

function removeCondition(idx: number) {
	draft.value.conditions.splice(idx, 1)
}

// 数值比较类运算需要数字输入提示
function isNumericOp(c: RuleCondition) {
	return c.kind === 'param' && ['gt', 'gte', 'lt', 'lte'].includes(c.op)
}

// 「存在 / 不存在」无比对值：条件体只剩两列（路径 + 匹配方式）
function isSimpleCond(c: RuleCondition) {
	return (c.kind === 'param' || c.kind === 'header') && c.op === 'exists'
}

function confirmEdit(done: (closed: boolean) => void) {
	if (draftError.value) {
		Message.warning(draftError.value)
		done(false)
		return
	}
	const rule = cloneRule(draft.value)
	if (!draftHasTime.value) rule.timezone = ''
	if (editingIndex.value < 0) rules.value.push(rule)
	else rules.value.splice(editingIndex.value, 1, rule)
	commitRules()
	done(true)
}
</script>

<template>
	<div class="rr-editor">
		<!-- 工具栏：添加规则 / 模板 / 模式切换 -->
		<div class="rr-toolbar">
			<template v-if="mode === 'visual'">
				<AButton size="small" type="outline" @click="openCreate">
					<template #icon><IconPlus /></template>
					添加规则
				</AButton>
				<ADropdown trigger="click" @select="addTemplate">
					<AButton size="small">
						常用模板
						<IconDown class="rr-down" />
					</AButton>
					<template #content>
						<ADoption v-for="tpl in RULE_TEMPLATES" :key="tpl.label" :value="tpl.label">{{ tpl.label }}</ADoption>
					</template>
				</ADropdown>
			</template>
			<ALink class="rr-mode-link" @click="mode === 'visual' ? switchToRaw() : switchToVisual()">
				{{ mode === 'visual' ? '表达式编辑' : '返回可视化编辑' }}
			</ALink>
		</div>

		<!-- 可视化：规则卡片列表 -->
		<template v-if="mode === 'visual'">
			<div v-if="rules.length === 0" class="rr-empty">暂无请求规则，所有请求按基础公式计价</div>
			<div v-else class="rr-list">
				<div v-for="(rule, idx) in rules" :key="idx" class="rr-card">
					<span class="rr-mult" :class="multiplierClass(rule.multiplier)">×{{ rule.multiplier }}</span>
					<span class="rr-desc">{{ describeRule(rule) }}</span>
					<span class="rr-ops">
						<AButton size="mini" type="text" @click="openEdit(idx)"><template #icon><IconEdit /></template></AButton>
						<APopconfirm content="删除这条规则？" @ok="removeRule(idx)">
							<AButton size="mini" type="text" status="danger"><template #icon><IconDelete /></template></AButton>
						</APopconfirm>
					</span>
				</div>
				<div class="rr-hint">同时命中的规则连乘（如 ×2 与 ×0.8 同时命中 = ×1.6），未命中的规则按 ×1</div>
			</div>
		</template>

		<!-- 表达式模式 -->
		<template v-else>
			<AAlert v-if="parseError" type="warning" class="rr-alert">
				当前表达式无法用可视化编辑器表示（{{ parseError }}），请直接编辑表达式
			</AAlert>
			<ATextarea
				:model-value="rawText"
				:auto-size="{ minRows: 2, maxRows: 8 }"
				placeholder="每条规则写成 (条件 ? 倍数 : 1)，多条用 * 连乘，如 (param(&quot;service_tier&quot;) == &quot;priority&quot; ? 2 : 1) * (hour() >= 23 || hour() < 8 ? 0.8 : 1)；留空 = 无请求规则"
				class="rr-textarea"
				@update:model-value="onRawInput"
			/>
		</template>

		<!-- 规则编辑弹窗：条件行三列网格（来源定宽 + 条件体自适应 + 删除定宽），长说明进 tooltip；
		     宽度按 min(720px, 100vw-32px) 自适应窗口，弹窗体超高时内部滚动，
		     条件再多也不挤压其余字段、不产生横向滚动条 -->
		<AModal
			v-model:visible="editorVisible"
			:title="editingIndex < 0 ? '添加请求规则' : '编辑请求规则'"
			:width="'min(720px, calc(100vw - 32px))'"
			:on-before-ok="confirmEdit"
			:body-style="{ maxHeight: '62vh', overflowY: 'auto' }"
			unmount-on-close
		>
			<div class="rr-form">
				<!-- 倍数与时区按可用宽度自动并排/堆叠（无时区条件时倍数独占整行），压缩纵向空间 -->
				<div class="rr-row2">
					<div class="rr-field">
						<div class="rr-label">倍数</div>
						<div class="rr-inline">
							<AInputNumber v-model="draft.multiplier" :min="0" :step="0.1" class="rr-mult-input" />
							<span class="rr-sub">大于 1 加价 · 小于 1 折扣</span>
						</div>
					</div>
					<div v-if="draftHasTime" class="rr-field">
						<div class="rr-label">时区</div>
						<ASelect v-model="draft.timezone" allow-clear allow-create allow-search placeholder="平台计费时区（默认）" class="rr-tz">
							<AOption v-for="tz in timezoneOptions" :key="tz" :value="tz">{{ tz }}</AOption>
						</ASelect>
					</div>
				</div>

				<div class="rr-field">
					<!-- 添加入口收进标题行，省掉整行虚线按钮 -->
					<div class="rr-conds-head">
						<span class="rr-label">条件（需同时满足）</span>
						<ADropdown trigger="click" @select="addCondition">
							<AButton size="mini" type="text">
								<template #icon><IconPlus /></template>
								添加条件
							</AButton>
							<template #content>
								<ADoption v-for="k in kindOptions" :key="k.value" :value="k.value">{{ k.label }}</ADoption>
							</template>
						</ADropdown>
					</div>
					<div class="rr-conds">
						<div v-if="draft.conditions.length === 0" class="rr-empty-cond">尚无条件，规则至少需要一个条件</div>
						<div
							v-for="(cond, idx) in draft.conditions"
							:key="idx"
							class="rr-cond"
							:class="`rr-cond--${cond.kind}`"
						>
							<ASelect
								:model-value="cond.kind"
								:options="kindOptions"
								size="small"
								class="rr-kind"
								@update:model-value="(v: any) => changeKind(idx, v)"
							/>
							<div class="rr-cond-body" :class="{ 'rr-cond-body--simple': isSimpleCond(cond) }">
								<ACheckboxGroup v-if="cond.kind === 'weekday'" v-model="cond.days" :options="weekdayOptions" class="rr-weekdays" />
								<template v-else-if="cond.kind === 'time'">
									<ATimePicker v-model="cond.start" format="HH:mm" size="small" :allow-clear="false" class="rr-time" />
									<span class="rr-dash">至</span>
									<ATimePicker v-model="cond.end" format="HH:mm" size="small" :allow-clear="false" class="rr-time" />
									<ATooltip content="结束早于开始 = 跨午夜时段；结束选 00:00 = 到当天结束">
										<span class="rr-tip"><IconQuestionCircle /></span>
									</ATooltip>
								</template>
								<template v-else-if="cond.kind === 'param'">
									<AInput v-model="cond.path" size="small" placeholder="参数路径，如 service_tier" class="rr-grow" />
									<ASelect v-model="cond.op" :options="paramOpOptions" size="small" class="rr-op" />
									<AInput
										v-if="cond.op !== 'exists'"
										v-model="cond.value"
										size="small"
										:placeholder="isNumericOp(cond) ? '数字' : '比对值'"
										class="rr-grow"
									/>
								</template>
								<template v-else-if="cond.kind === 'header'">
									<AInput v-model="cond.name" size="small" placeholder="请求头名，如 anthropic-beta" class="rr-grow" />
									<ASelect v-model="cond.op" :options="headerOpOptions" size="small" class="rr-op" />
									<AInput v-if="cond.op !== 'exists'" v-model="cond.value" size="small" placeholder="比对值" class="rr-grow" />
								</template>
							</div>
							<AButton size="mini" type="text" status="danger" class="rr-del" @click="removeCondition(idx)">
								<template #icon><IconDelete /></template>
							</AButton>
						</div>
					</div>
				</div>

				<div class="rr-field">
					<div class="rr-label">生成的表达式</div>
					<code v-if="draftPreview" class="rr-preview">{{ draftPreview }}</code>
					<div v-else class="rr-preview rr-preview--error">{{ draftError }}</div>
				</div>
			</div>
		</AModal>
	</div>
</template>

<style scoped>
.rr-editor {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.rr-toolbar {
	display: flex;
	align-items: center;
	gap: 8px;
	flex-wrap: wrap;
}

.rr-down {
	margin-left: 4px;
	font-size: 12px;
}

.rr-mode-link {
	margin-left: auto;
	font-size: 12px;
}

.rr-empty {
	padding: 12px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-align: center;
	border: 1px dashed var(--color-border-2);
	border-radius: 6px;
}

.rr-list {
	display: flex;
	flex-direction: column;
	gap: 6px;
}

.rr-card {
	display: flex;
	align-items: center;
	gap: 10px;
	padding: 8px 10px;
	background: var(--color-bg-2);
	border: 1px solid var(--color-border-2);
	border-radius: 6px;
}

.rr-mult {
	flex-shrink: 0;
	min-width: 52px;
	padding: 2px 8px;
	font-family: 'JetBrains Mono', Consolas, monospace;
	font-size: 13px;
	font-weight: 600;
	text-align: center;
	border-radius: 4px;
}

.rr-mult--up {
	color: rgb(var(--orange-6));
	background: rgb(var(--orange-1));
}

.rr-mult--down {
	color: rgb(var(--green-6));
	background: rgb(var(--green-1));
}

.rr-desc {
	flex: 1;
	min-width: 0;
	font-size: 13px;
	color: var(--ta-text-primary);
	word-break: break-all;
}

.rr-ops {
	display: flex;
	flex-shrink: 0;
	gap: 2px;
}

.rr-hint {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.rr-alert {
	margin: 0;
}

.rr-textarea :deep(textarea) {
	font-family: 'JetBrains Mono', Consolas, Monaco, monospace;
	font-size: 13px;
}

/* 编辑弹窗：字段纵向堆叠，倍数与时区并排压缩高度；弹窗体超高时内部滚动 */
.rr-form {
	display: flex;
	flex-direction: column;
	gap: 14px;
}

/* 倍数 / 时区：按可用宽度自动决定并排还是堆叠，不依赖视口断点
   （弹窗宽度是 min(720px, 100vw-32px)，用视口断点会在窄窗口下误判） */
.rr-row2 {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
	gap: 12px;
}

/* 无时区条件时只剩倍数一项，独占整行而不是留半个空档 */
.rr-row2 > :only-child {
	grid-column: 1 / -1;
}

.rr-field {
	display: flex;
	flex-direction: column;
	gap: 6px;
	min-width: 0;
}

.rr-label {
	font-size: 13px;
	font-weight: 600;
	color: var(--ta-text-primary);
}

.rr-inline {
	display: flex;
	align-items: center;
	gap: 8px;
	min-width: 0;
}

.rr-mult-input {
	flex: 0 0 120px;
	width: 120px;
}

.rr-sub {
	min-width: 0;
	overflow: hidden;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-overflow: ellipsis;
	white-space: nowrap;
}

.rr-conds-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
}

.rr-conds {
	display: flex;
	flex-direction: column;
	gap: 6px;
}

.rr-empty-cond {
	padding: 8px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
	text-align: center;
	border: 1px dashed var(--color-border-2);
	border-radius: 6px;
}

/* 条件行：来源（定宽）+ 条件体（自适应）+ 删除（定宽）三列网格。
   用 grid + minmax(0, 1fr) 而非 flex-wrap：flex 下固定像素的子项不收缩，
   来源下拉一旦吃满整行，条件体就会被挤到行外溢出，弹窗体出现横向滚动条 */
.rr-cond {
	display: grid;
	grid-template-columns: 108px minmax(0, 1fr) 24px;
	align-items: center;
	gap: 8px;
	padding: 6px 8px;
	background: var(--color-fill-1);
	border-radius: 6px;
}

.rr-kind {
	width: 100%;
}

.rr-cond-body {
	display: grid;
	align-items: center;
	gap: 8px;
	min-width: 0;
}

/* 参数/请求头：路径 + 匹配方式 + 比对值；两输入框均分剩余空间 */
.rr-cond--param .rr-cond-body,
.rr-cond--header .rr-cond-body {
	grid-template-columns: minmax(0, 1fr) 108px minmax(0, 1fr);
}

/* 「存在」判断无比对值：路径独占剩余空间 */
.rr-cond--param .rr-cond-body--simple,
.rr-cond--header .rr-cond-body--simple {
	grid-template-columns: minmax(0, 1fr) 108px;
}

/* 时间段：起止固定窄列靠左排列，允许收缩到 84px 也不会顶出容器 */
.rr-cond--time .rr-cond-body {
	grid-template-columns: minmax(84px, 100px) auto minmax(84px, 100px) auto;
	justify-content: start;
}

/* 星期：七项自换行 */
.rr-cond--weekday .rr-cond-body {
	display: flex;
	flex-wrap: wrap;
	gap: 4px 10px;
}

.rr-weekdays {
	min-width: 0;
}

.rr-grow {
	width: 100%;
	min-width: 0;
}

.rr-op {
	width: 100%;
}

.rr-time {
	width: 100%;
}

.rr-dash {
	color: var(--ta-text-tertiary);
}

.rr-tip {
	display: inline-flex;
	align-items: center;
	color: var(--ta-text-tertiary);
	cursor: help;
}

.rr-del {
	justify-self: end;
}

.rr-tz {
	width: 100%;
}

.rr-preview {
	display: block;
	padding: 8px 10px;
	font-family: 'JetBrains Mono', Consolas, monospace;
	font-size: 12px;
	color: var(--ta-text-secondary);
	word-break: break-all;
	background: var(--color-fill-2);
	border-radius: 6px;
}

.rr-preview--error {
	color: rgb(var(--orange-6));
}

/* 窄窗口（弹窗被 min(720px, 100vw-32px) 压到 ≤520px）时，
   参数类条件行把「路径 / 匹配方式 / 比对值」竖排，避免三列一起被压扁 */
@media (max-width: 520px) {
	.rr-cond--param .rr-cond-body,
	.rr-cond--header .rr-cond-body {
		grid-template-columns: minmax(0, 1fr);
	}

	.rr-cond--param .rr-cond-body--simple,
	.rr-cond--header .rr-cond-body--simple {
		grid-template-columns: minmax(0, 1fr) 108px;
	}
}
</style>
