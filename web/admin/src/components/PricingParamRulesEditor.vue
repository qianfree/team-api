<script lang="ts">
// 行数据结构导出（<script setup> 不支持 export，类型声明放前置块）
export interface ParamConditionRow {
	path: string
	match: string // has_value | equals | gt | gte | lt | lte | between
	value: string // 输入态字符串；between 逗号分隔；提交时按匹配方式转换
}

export interface ParamRuleRow {
	conditions: ParamConditionRow[]
	multiplier: number
	note: string
}

// 无需比对值的匹配方式（has_value 只判字段存在）
function matchNeedsValue(match: string): boolean {
	return match !== 'has_value'
}

// 条件输入态 → 提交值（数字类强转、between 转双元素数组、equals 保原样）
function conditionValuePayloadFrom(cond: ParamConditionRow): any {
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

// API 规则列表 → 行编辑态。父级在编辑器组件未挂载时（如二层弹窗未打开/折叠区未展开）
// 也用本函数直接填充行数组，与组件内 loadParamMultipliers 共用同一换算
export function paramRuleRowsFromAPI(list: any[] | null | undefined): ParamRuleRow[] {
	const rows: ParamRuleRow[] = []
	if (!Array.isArray(list)) return rows
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
		if (row.conditions.length > 0) rows.push(row)
	}
	return rows
}

// 行编辑态 → 提交载荷（含校验）。error 非空=校验失败文案（提示方式由调用方决定）
export function paramRulePayloadFrom(rows: ParamRuleRow[]): { payload?: any[]; error?: string } {
	const payload: any[] = []
	for (const [i, rule] of rows.entries()) {
		if (rule.multiplier <= 0 || rule.multiplier > 100) {
			return { error: `参数倍率规则 ${i + 1} 倍率需在 (0, 100] 区间` }
		}
		if (rule.conditions.length === 0) {
			return { error: `参数倍率规则 ${i + 1} 至少需要一个条件` }
		}
		const conditions: any[] = []
		for (const [j, cond] of rule.conditions.entries()) {
			const path = cond.path.trim()
			if (!path) {
				return { error: `参数倍率规则 ${i + 1} 条件 ${j + 1} 路径为空` }
			}
			const value = conditionValuePayloadFrom(cond)
			if (matchNeedsValue(cond.match) && (value === null || value === '')) {
				return { error: `参数倍率规则 ${i + 1} 条件 ${j + 1} 需要填写比对值` }
			}
			conditions.push(matchNeedsValue(cond.match) ? { path, match: cond.match, value } : { path, match: cond.match })
		}
		// 命中说明：空 = 默认「规则N」（随序号，删除上方规则后自动顺延）
		payload.push({ conditions, multiplier: rule.multiplier, note: rule.note.trim() || `规则 ${i + 1}` })
	}
	return { payload }
}
</script>

<script setup lang="ts">
// 参数倍率编辑器（主弹窗与官方定价弹窗共用）：
// 按归一化任务体参数匹配，命中规则连乘；规则数组由父级持有（reactive），
// 编辑/校验/载荷/试匹配逻辑内聚在本组件。
import { ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import { IconDelete } from '@arco-design/web-vue/es/icon'

const props = withDefaults(
	defineProps<{
		rules: ParamRuleRow[]
		title?: string
		hint?: string
	}>(),
	{
		title: '参数倍率',
		hint: '按归一化任务体参数匹配（如含参考图 / 时长档位 / 视频输入），命中规则连乘；所有计费模式共享',
	},
)

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
	props.rules.push({ conditions: [{ path: '', match: 'has_value', value: '' }], multiplier: 1, note: '' })
}

function removeParamRule(index: number) {
	props.rules.splice(index, 1)
}

function addParamCondition(ruleIndex: number) {
	props.rules[ruleIndex].conditions.push({ path: '', match: 'has_value', value: '' })
}

function removeParamCondition(ruleIndex: number, condIndex: number) {
	props.rules[ruleIndex].conditions.splice(condIndex, 1)
}

// 模板绑定：模板只认 setup 绑定，前置块的 matchNeedsValue 经此别名暴露
const conditionNeedsValue = matchNeedsValue

// 行编辑态 → 提交载荷；校验失败 Message 提示并返回 null（委托前置块纯函数）
function buildParamMultipliersPayload(): any[] | null {
	const result = paramRulePayloadFrom(props.rules)
	if (result.error) {
		Message.warning(result.error)
		return null
	}
	return result.payload ?? []
}

// 回显：后端规则 → 行编辑态（委托前置块纯函数，与父级直接填充行数组共用换算）
function loadParamMultipliers(list: any[] | null | undefined) {
	const rows = paramRuleRowsFromAPI(list)
	props.rules.length = 0
	props.rules.push(...rows)
}

function resetParamRules() {
	props.rules.length = 0
	tryMatchInput.value = ''
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
	for (const rule of props.rules) {
		const allHit = rule.conditions.every((c) =>
			conditionHit({ path: c.path, match: c.match, value: conditionValuePayloadFrom(c) }, root),
		)
		if (allHit) {
			total *= rule.multiplier || 1
			hits.push({ label: rule.note || rule.conditions.map((c) => c.path).join(' & '), multiplier: rule.multiplier })
		}
	}
	return { hits, total }
}

defineExpose({ loadParamMultipliers, resetParamRules, buildParamMultipliersPayload })
</script>

<template>
	<div class="editor-section">
		<div class="editor-section-header">
			<h3>{{ title }}</h3>
			<span class="section-hint">{{ hint }}</span>
		</div>
		<div v-for="(rule, ruleIndex) in rules" :key="ruleIndex" class="tier-card">
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
		<div v-if="rules.length" class="param-try-match">
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

.combo-value {
	color: var(--ta-text-primary);
	font-family: monospace;
	text-align: right;
}

.combo-preview-hint {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}
</style>
