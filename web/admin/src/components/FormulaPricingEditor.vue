<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { currencySymbol } from '@/composables/useCurrency'
import RequestRulesEditor from './RequestRulesEditor.vue'

// 表达式计费编辑器（billing_mode=formula）：价格完全由公式决定——
// 基础公式从 usage 变量环境求出「每 1M token 口径的价格系数」，金额 = 系数 / 1e6 × 租户乘数；
// 请求规则（可选）与基础公式相乘，组合为 (公式) * (规则)，用 header()/param()/时间函数
// 按请求头/请求体参数/时段调倍率（formula 模型不再单独配置时段定价与参数倍率，全部在公式内表达），
// 命中状态随账单留痕。本组件自治校验：保存前父组件调 validate()，未通过中止保存
// （后端保存链路同口径烟测兜底）

const props = defineProps<{
	modelValue: string
	modelValueRules?: string
}>()

const emit = defineEmits<{
	'update:modelValue': [value: string]
	'update:modelValueRules': [value: string]
}>()

const formulaText = ref(props.modelValue)
const rulesText = ref(props.modelValueRules || '')
watch(
	() => props.modelValue,
	(v) => {
		formulaText.value = v
	},
)
watch(
	() => props.modelValueRules,
	(v) => {
		rulesText.value = v || ''
	},
)
watch(formulaText, (v) => {
	emit('update:modelValue', v)
	// 公式变更后上次校验结果失效
	invalidateValidation()
})
watch(rulesText, (v) => {
	emit('update:modelValueRules', v)
	invalidateValidation()
})

function invalidateValidation() {
	validatedMark.value = ''
	validateError.value = ''
	validateResults.value = null
	usedVars.value = []
}

// 校验状态：validatedMark 记录「已通过校验的公式+规则文本」，保存时不一致须重新校验
const validating = ref(false)
const validatedMark = ref('')
const validateError = ref('')
const validateResults = ref<any[] | null>(null)
const usedVars = ref<string[]>([])

// 变量速查表（与后端 billing/formula 包 knownVars 同步维护）
const varDocs: Array<{ name: string; desc: string }> = [
	{ name: 'p', desc: '输入 token（未单独计价的缓存/图片/音频都计入 p 按输入价计；写了 cr/cc/img/ai 后对应部分自动从 p 扣除）' },
	{ name: 'c', desc: '输出 token（引用 rt/img_o/ao 时自动扣除对应部分）' },
	{ name: 'ctx', desc: '完整输入上下文 token（含缓存，用于阶梯档位条件判断）' },
	{ name: 'cr', desc: '缓存读取（命中）token' },
	{ name: 'cc', desc: '缓存写入 5m token（Claude ephemeral_5m / OpenAI cache_write）；未写 cc1h 时 1h 写入按 cc 单价 ×1.6 计' },
	{ name: 'cc1h', desc: '缓存写入 1h token（Claude ephemeral_1h）；仅当 1h 写入价不是 cc 的 1.6 倍时才需单独写' },
	{ name: 'img / img_o', desc: '图片输入 / 输出 token' },
	{ name: 'ai / ao', desc: '音频输入 / 输出 token' },
	{ name: 'rt', desc: '推理输出 token（reasoning）' },
	{ name: 'image_count', desc: '图片张数（图片协议顶层 n / 异步任务请求张数）' },
	{ name: 'seconds', desc: '任务计费秒数（视频生成等时长类任务：预扣按请求时长，结算按官方实际秒数多退少补；同步请求恒 0）' },
]

// 已注册 usage facts key（校验接口下发 known_fact_keys；空 = 未请求过校验）
const knownFactKeys = ref<Record<string, string>>({})

// 示例模板（点击填充到输入框）。多档位一律用 tier("档位名", 计价式) 标记：
// 账单据此展示命中档位，计价式为「变量 × 单价」之和时还能拆出逐项费用明细
const formulaTemplates: Array<{ label: string; expr: string }> = [
	{ label: '纯 token 单价', expr: 'tier("base", p * 5 + c * 20)' },
	{ label: '含缓存计价', expr: 'tier("base", p * 3 + c * 15 + cr * 0.3 + cc * 3.75)' },
	{ label: '推理 token 差价', expr: 'tier("base", p * 2 + c * 10 + rt * 30)' },
	{ label: '音频多模态', expr: 'tier("base", p * 5 + c * 30 + img * 8 + ai * 10 + ao * 20)' },
	{ label: '图片按张计费', expr: 'tier("per_image", fixed(0.04) * image_count)' },
	{
		label: '长上下文两档',
		expr: 'ctx <= 200000\n  ? tier("standard", p * 3 + c * 15 + cr * 0.3 + cc * 3.75)\n  : tier("long_context", p * 6 + c * 22.5 + cr * 0.6 + cc * 7.5)',
	},
	{
		label: '三档阶梯',
		expr: 'ctx <= 32000\n  ? tier("short", p * 1.2 + c * 6 + cr * 0.24 + cc * 1.5)\n  : (ctx <= 128000\n    ? tier("mid", p * 2.4 + c * 12 + cr * 0.48 + cc * 3)\n    : tier("long", p * 3 + c * 15 + cr * 0.6 + cc * 3.75))',
	},
	{ label: 'token + 按次附加费', expr: 'tier("base", p * 5 + c * 20 + fixed(0.001))' },
	{ label: '语音按秒计费', expr: 'tier("per_second", fixed(0.4) * u("audio_seconds"))' },
	{ label: '视频任务按秒', expr: 'tier("per_second", fixed(0.4) * seconds)' },
]

function applyTemplate(expr: string | number | boolean | Record<string, any> | Array<string | number | boolean | Record<string, any>>) {
	if (typeof expr !== 'string') return
	formulaText.value = expr
	Message.info('模板已填充，可编辑后点「校验并试算」')
}

// ============ 模拟请求（试算请求规则命中） ============
// 时间按浏览器本地时间选择，提交时转 RFC3339；留空 = 当前时刻。
// 请求头每行「名称: 值」，请求体为 JSON；三者套用到全部预置样例
const simAt = ref('')
const simHeadersText = ref('')
const simBodyText = ref('')

function parseSimHeaders(text: string): Record<string, string> {
	const headers: Record<string, string> = {}
	for (const line of text.split('\n')) {
		const idx = line.indexOf(':')
		if (idx <= 0) continue
		const name = line.slice(0, idx).trim()
		if (name) headers[name] = line.slice(idx + 1).trim()
	}
	return headers
}

// buildSimulate 组装模拟请求；请求体 JSON 非法时返回 null（调用方中止校验并提示）。
// 即使未填写也发送空对象：后端据此按平台计费时区求值时间函数（预置样例默认无请求上下文）
function buildSimulate(): Record<string, any> | null {
	const sim: Record<string, any> = {}
	if (simAt.value) {
		const t = new Date(simAt.value.replace(' ', 'T'))
		if (!Number.isNaN(t.getTime())) sim.eval_at = t.toISOString()
	}
	const headers = parseSimHeaders(simHeadersText.value)
	if (Object.keys(headers).length > 0) sim.headers = headers
	const body = simBodyText.value.trim()
	if (body) {
		try {
			sim.body = JSON.parse(body)
		} catch {
			return null
		}
	}
	return sim
}

// 模拟请求下的规则命中汇总（各样例请求上下文相同，取首个样例的命中表）
const simMatchedRules = computed<any[]>(() => {
	const rules: any[] = validateResults.value?.[0]?.rules || []
	return rules.filter((r) => r.matched)
})
const simRuleCount = computed(() => (validateResults.value?.[0]?.rules || []).length)
const simRuleProduct = computed(() => Number(simMatchedRules.value.reduce((acc, r) => acc * r.multiplier, 1).toFixed(6)))

async function doValidate(): Promise<boolean> {
	const src = formulaText.value.trim()
	const rules = rulesText.value.trim()
	if (!src) {
		validateError.value = '计费公式不能为空'
		Message.warning('计费公式不能为空')
		return false
	}
	const simulate = buildSimulate()
	if (!simulate) {
		validateError.value = '模拟请求体不是合法的 JSON'
		Message.warning('模拟请求体不是合法的 JSON')
		return false
	}
	validating.value = true
	try {
		const res: any = await request.post('/admin/models/pricing/validate-formula', {
			formula: src,
			formula_rules: rules || undefined,
			simulate,
		})
		const data = res.data?.data
		if (!data?.valid) {
			validateError.value = data?.error || '公式校验未通过'
			validateResults.value = data?.results || null
			usedVars.value = data?.vars || []
			// 校验失败即作废上一次通过标记，避免失败态与"校验通过"提示同时出现
			validatedMark.value = ''
			Message.error(`公式校验未通过：${validateError.value}`)
			return false
		}
		validateError.value = ''
		validateResults.value = data?.results || []
		usedVars.value = data?.vars || []
		knownFactKeys.value = data?.known_fact_keys || {}
		validatedMark.value = `${src}|||${rules}`
		Message.success('公式校验通过')
		return true
	} catch {
		// 错误提示由 axios 拦截器统一处理
		return false
	} finally {
		validating.value = false
	}
}

// 保存前置校验入口：公式/规则未变更且已通过 → 直接放行；否则重新校验
async function validate(): Promise<boolean> {
	const mark = `${formulaText.value.trim()}|||${rulesText.value.trim()}`
	if (formulaText.value.trim() && validatedMark.value === mark) {
		return true
	}
	return await doValidate()
}

defineExpose({ validate })

// 环境快照展示：只显示公式引用的变量（与后端返回一致，空环境不渲染行）。
// header:/param: 值可能是字符串（请求探测值），数字才做千分位
function envEntries(env: Record<string, any> | undefined): Array<[string, string]> {
	if (!env) return []
	return Object.entries(env)
		.filter(([, v]) => (typeof v === 'number' ? v > 0 : v !== '' && v != null))
		.map(([k, v]) => [k, typeof v === 'number' ? v.toLocaleString() : String(v)])
}

// 命中规则展示
function matchedRules(rules: any[] | undefined): any[] {
	return rules || []
}

function fmtAmount(v: number): string {
	return `${currencySymbol.value} ${v.toFixed(6)}`
}

// 已注册 facts key 表项（接口下发优先；未校验前用本地兜底，与后端注册表同步维护）
const localFactKeys: Record<string, string> = {
	'audio_seconds': '音频时长秒数（STT/翻译 verbose_json.duration，OpenAI 渠道填充）',
	'audio_bytes': '音频响应字节数（TTS 二进制响应实测大小，OpenAI 渠道填充）',
}
const factKeyEntries = computed<Array<[string, string]>>(() => {
	const src = Object.keys(knownFactKeys.value).length > 0 ? knownFactKeys.value : localFactKeys
	return Object.entries(src).sort(([a], [b]) => a.localeCompare(b))
})

</script>

<template>
	<div class="formula-pricing-editor">
		<!-- 头部沿用弹窗 section 语言：标题 + 右侧动作；口径说明降为一行小字（细节进速查） -->
		<div class="editor-section-header">
			<h3>计费公式</h3>
			<div class="fx-actions">
				<ASelect size="small" placeholder="示例模板…" class="fx-template-select" @change="applyTemplate">
					<AOption v-for="tpl in formulaTemplates" :key="tpl.expr" :value="tpl.expr">{{ tpl.label }}</AOption>
				</ASelect>
				<AButton size="small" type="primary" :loading="validating" @click="doValidate">校验并试算</AButton>
			</div>
		</div>
		<div class="fx-caption">
			金额 = 系数 ÷ 1M × 租户乘数；<code>fixed(x)</code> = 每次请求 x {{ currencySymbol }}；
			多档位用 <code>tier("档位名", 计价式)</code> 标记，账单可展示命中档位与逐项明细；
			时段 / 请求参数条件写在下方「请求规则」（hour()/param()），不再单独配置时段定价与参数倍率
		</div>

		<!-- 基础公式 -->
		<div class="fx-card">
			<div class="fx-card-head">
				<span class="fx-card-label">基础公式<em class="fx-req">*</em></span>
				<span class="fx-card-sub">token 系数 / 按次 / 按秒 / 长上下文档位</span>
			</div>
			<ATextarea
				v-model="formulaText"
				:auto-size="{ minRows: 3, maxRows: 8 }"
				placeholder='例如：ctx <= 128000 ? tier("standard", p * 5 + c * 20) : tier("long_context", p * 10 + c * 40)'
				class="fx-textarea"
			/>
		</div>

		<!-- 请求规则（可选） -->
		<div class="fx-card">
			<div class="fx-card-head">
				<span class="fx-card-label">
					请求规则
					<ATag size="small" class="fx-optional">可选</ATag>
				</span>
				<span class="fx-card-sub">按时间 / 请求头 / 请求参数加价或打折，命中状态随账单留痕</span>
			</div>
			<RequestRulesEditor v-model="rulesText" />
		</div>

		<!-- 模拟请求：套用到全部预置样例，试算请求规则命中（默认折叠） -->
		<ACollapse :default-active-key="[]" class="fx-sim">
			<ACollapseItem key="sim" header="模拟请求（试算请求规则是否命中）">
				<div class="fx-sim-grid">
					<div class="fx-sim-field">
						<div class="fx-sim-label">请求时间（浏览器本地时间，留空 = 当前时刻）</div>
						<ADatePicker v-model="simAt" show-time format="YYYY-MM-DD HH:mm" value-format="YYYY-MM-DD HH:mm" class="fx-sim-date" />
					</div>
					<div class="fx-sim-field">
						<div class="fx-sim-label">请求头（每行「名称: 值」）</div>
						<ATextarea v-model="simHeadersText" :auto-size="{ minRows: 2, maxRows: 5 }" placeholder="anthropic-beta: fast-mode-2025" class="fx-textarea" />
					</div>
					<div class="fx-sim-field fx-sim-field--full">
						<div class="fx-sim-label">请求体 JSON</div>
						<ATextarea v-model="simBodyText" :auto-size="{ minRows: 2, maxRows: 6 }" placeholder='{"service_tier": "priority", "duration": 12}' class="fx-textarea" />
					</div>
				</div>
				<div class="fx-sim-hint">填写后点「校验并试算」，下方试算结果按该请求计算规则命中与金额</div>
			</ACollapseItem>
		</ACollapse>

		<!-- 校验反馈：失败走 AAlert（可直接展示后端中文报错），通过给一条持久确认 -->
		<AAlert v-if="validateError" type="error" class="fx-feedback" :title="`校验未通过：${validateError}`" />
		<div v-else-if="validatedMark" class="fx-feedback fx-feedback--ok">
			<ATag color="green" size="small">校验通过</ATag>
			<span class="fx-feedback-text">
				引用 {{ usedVars.length }} 个变量<template v-if="validateResults && validateResults.length">，试算 {{ validateResults.length }} 条样例</template>
			</span>
		</div>

		<!-- 请求规则命中汇总（模拟请求下，各样例的请求上下文相同） -->
		<div v-if="validatedMark && simRuleCount > 0" class="fx-sim-summary">
			<span class="fx-vars-label">规则命中：</span>
			<template v-if="simMatchedRules.length > 0">
				<ATag v-for="(r, i) in simMatchedRules" :key="i" color="green" size="small" :title="r.cond">×{{ r.multiplier }} {{ r.desc || '' }}</ATag>
				<span class="fx-feedback-text">命中 {{ simMatchedRules.length }}/{{ simRuleCount }} 条，规则倍率合计 ×{{ simRuleProduct }}</span>
			</template>
			<span v-else class="fx-feedback-text">{{ simRuleCount }} 条规则均未命中（×1）</span>
		</div>

		<!-- 引用变量 chips -->
		<div v-if="usedVars.length > 0" class="fx-vars">
			<span class="fx-vars-label">引用变量：</span>
			<ATag v-for="v in usedVars" :key="v" color="arcoblue" size="small">{{ v }}</ATag>
		</div>

		<!-- 试算结果表 -->
		<ATable
			v-if="validateResults && validateResults.length > 0"
			:data="validateResults"
			:pagination="false"
			size="small"
			class="fx-results"
		>
			<template #columns>
				<ATableColumn title="样例" data-index="name" :width="100" />
				<ATableColumn title="变量取值（非零）" :width="220">
					<template #cell="{ record }">
						<template v-if="envEntries(record.env).length > 0">
							<ATag v-for="[k, v] in envEntries(record.env)" :key="k" size="small" class="env-tag">
								{{ k }}={{ v }}
							</ATag>
						</template>
						<span v-else class="env-empty">（全零）</span>
					</template>
				</ATableColumn>
				<ATableColumn title="系数" :width="110">
					<template #cell="{ record }">
						<span class="mono">{{ record.factor.toLocaleString() }}</span>
					</template>
				</ATableColumn>
				<ATableColumn title="命中规则" :width="200">
					<template #cell="{ record }">
						<template v-if="matchedRules(record.rules).length > 0">
							<div v-for="(rule, i) in matchedRules(record.rules)" :key="i" class="rule-trace">
								<ATag :color="rule.matched ? 'green' : 'gray'" size="small">
									{{ rule.matched ? `×${rule.multiplier}` : '×1' }}
								</ATag>
								<span class="rule-cond" :title="rule.cond">{{ rule.desc || rule.cond }}</span>
							</div>
						</template>
						<span v-else class="env-empty">（无规则）</span>
					</template>
				</ATableColumn>
				<ATableColumn title="试算金额（折扣前）" :width="164">
					<template #cell="{ record }">
						<span class="mono">{{ fmtAmount(record.amount) }}</span>
					</template>
				</ATableColumn>
			</template>
		</ATable>

		<!-- 变量速查（默认折叠） -->
		<ACollapse :default-active-key="[]" class="fx-docs">
			<ACollapseItem key="vars" header="变量与函数速查">
				<table class="vars-table">
					<tbody>
						<tr v-for="doc in varDocs" :key="doc.name">
							<td class="mono vars-name">{{ doc.name }}</td>
							<td>{{ doc.desc }}</td>
						</tr>
						<tr>
							<td class="mono vars-name">fixed(x)</td>
							<td>按次金额：每次请求 x {{ currencySymbol }}（如 fixed(0.04) * image_count 按张放大）</td>
						</tr>
						<tr>
							<td class="mono vars-name">tier("名称", 计价式)</td>
							<td>
								标记计价档位：值等于计价式，账单展示命中的档位名。计价式写成「变量 × 单价」之和
								（可含 fixed()）时，账单还会逐项列出「数量 × 单价 = 费用」明细；
								例：ctx &lt;= 128000 ? tier("≤128K", p * 5 + c * 20) : tier("&gt;128K", p * 10 + c * 40)
							</td>
						</tr>
						<tr>
							<td class="mono vars-name">u("key")</td>
							<td>
								取 usage 旁路计量值（非 token 计量，如音频秒数）。key 必须已注册（见下表），
								上游未上报时取 0；预扣阶段无该值、结算按真实值补差。
								例：u("audio_seconds") * 400000 = 每秒 0.4 {{ currencySymbol }}
							</td>
						</tr>
						<tr>
							<td class="mono vars-name">min/max/abs/ceil/floor</td>
							<td>数值辅助函数，如 max(ctx - 128000, 0)</td>
						</tr>
						<tr>
							<td class="mono vars-name">header("key")</td>
							<td>
								请求感知：读客户端请求头值（键大小写不敏感，缺失为空串）。
								仅限请求规则区使用；key 须为字符串字面量；值为字符串，只能做等于/包含匹配。
								例：(has(header("anthropic-beta"), "fast-mode") ? 6 : 1)
							</td>
						</tr>
						<tr>
							<td class="mono vars-name">param("path")</td>
							<td>
								请求感知：按点分路径读请求体 JSON 值（如 param("service_tier")，支持
								metadata.image 嵌套与 content[*].type 通配；缺失为 nil）。
								仅限请求规则区；路径须为字符串字面量。异步任务在提交时冻结探测值，结算重放。
								与数字比较时写 (param("duration") ?? 0) >= 10（请求不带该参数时按 0 处理）
							</td>
						</tr>
						<tr>
							<td class="mono vars-name">has(v, "s")</td>
							<td>子串包含（header 值非精确匹配时用），如 has(header("x-beta"), "fast")</td>
						</tr>
						<tr>
							<td class="mono vars-name">hour("tz") / minute / weekday / month / day</td>
							<td>
								时间感知：请求受理时刻在指定时区的钟表时间（weekday 0=周日）。时区省略或非法
								取平台计费时区。预扣与结算同为受理时刻，跨小时边界不漂移。
								例：(hour() >= 23 || hour() &lt; 8 ? 0.8 : 1)；多条规则用 * 连乘，每条须带外层括号
							</td>
						</tr>
					</tbody>
				</table>
				<!-- 已注册 facts key（校验接口下发；本地兜底列表在接口调用前展示） -->
				<div v-if="factKeyEntries.length > 0" class="fact-keys">
					<div class="fact-keys-title">已注册 usage 字段（u() 可用 key）：</div>
					<table class="vars-table">
						<tbody>
							<tr v-for="[key, desc] in factKeyEntries" :key="key">
								<td class="mono vars-name">u("{{ key }}")</td>
								<td>{{ desc }}</td>
							</tr>
						</tbody>
					</table>
				</div>
			</ACollapseItem>
		</ACollapse>
	</div>
</template>

<style scoped>
.formula-pricing-editor {
	display: flex;
	flex-direction: column;
	gap: 12px;
}

/* 与弹窗其他 section 同一套标题语言（14px/600，右侧动作右对齐） */
.editor-section-header {
	display: flex;
	align-items: center;
	gap: 12px;
	flex-wrap: wrap;
}

.editor-section-header h3 {
	font-size: 14px;
	font-weight: 600;
	color: var(--ta-text-primary);
	margin: 0;
}

.fx-actions {
	display: flex;
	align-items: center;
	gap: 8px;
	margin-left: auto;
}

.fx-template-select {
	width: 168px;
}

/* 口径说明：纯文字一行，不再单独占一块灰底（细节进速查折叠） */
.fx-caption {
	font-size: 12px;
	color: var(--ta-text-tertiary);
	line-height: 1.7;
}

/* 卡片：与 tier-card 同构（fill-1 底 + 8px 圆角、无边框），保持弹窗内视觉统一 */
.fx-card {
	display: flex;
	flex-direction: column;
	gap: 8px;
	padding: 12px 16px;
	background: var(--color-fill-1);
	border-radius: 8px;
}

.fx-card-head {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
	flex-wrap: wrap;
}

.fx-card-label {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	font-size: 13px;
	font-weight: 600;
	color: var(--ta-text-primary);
}

.fx-card-sub {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.fx-req {
	color: rgb(var(--red-6));
	font-style: normal;
	margin-left: 2px;
}

.fx-optional {
	font-weight: 400;
}

.fx-textarea :deep(textarea) {
	font-family: 'JetBrains Mono', Consolas, Monaco, monospace;
	font-size: 13px;
}

/* 模拟请求：两列网格（时间 + 请求头），请求体独占一行；窄屏单列 */
.fx-sim-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 10px 12px;
}

.fx-sim-field {
	display: flex;
	flex-direction: column;
	gap: 4px;
	min-width: 0;
}

.fx-sim-field--full {
	grid-column: 1 / -1;
}

.fx-sim-label {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.fx-sim-date {
	width: 100%;
}

.fx-sim-hint {
	margin-top: 8px;
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.fx-sim-summary {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 6px;
}

@media (max-width: 720px) {
	.fx-sim-grid {
		grid-template-columns: 1fr;
	}
}

/* 校验反馈：失败态用 AAlert 撑满一行，通过态用一条轻量确认行 */
.fx-feedback {
	margin: 0;
}

.fx-feedback--ok {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 6px 12px;
	border-radius: 8px;
	background: var(--color-success-light-1);
}

.fx-feedback-text {
	font-size: 12px;
	color: var(--ta-text-secondary);
}

.fx-vars {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 4px;
}

.fx-vars-label {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.fx-results {
	margin-top: 2px;
}

.env-tag {
	margin: 0 4px 4px 0;
	font-family: 'JetBrains Mono', Consolas, monospace;
	font-size: 12px;
}

.env-empty {
	font-size: 12px;
	color: var(--ta-text-tertiary);
}

.rule-trace {
	display: flex;
	align-items: center;
	gap: 6px;
	margin-bottom: 2px;
}

.rule-cond {
	font-family: 'JetBrains Mono', Consolas, monospace;
	font-size: 12px;
	color: var(--ta-text-secondary);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
	max-width: 146px;
}

.fx-docs {
	border: none;
	background: transparent;
}

.mono {
	font-family: 'JetBrains Mono', Consolas, monospace;
}

.vars-table {
	width: 100%;
	font-size: 12px;
	border-collapse: collapse;
}

.vars-table td {
	padding: 4px 8px;
	border-bottom: 1px solid var(--color-fill-2);
	vertical-align: top;
}

.vars-name {
	white-space: nowrap;
	color: var(--color-text-2);
	width: 200px;
}

.fact-keys {
	margin-top: 8px;
}

.fact-keys-title {
	font-size: 12px;
	color: var(--color-text-3);
	margin-bottom: 4px;
}
</style>
