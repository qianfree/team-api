<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { currencySymbol } from '@/composables/useCurrency'

// 表达式计费编辑器（billing_mode=formula）：
// 公式从 usage 变量环境取值求出「每 1M token 口径的价格系数」，
// 金额 = 系数 / 1e6 × 租户乘数 × 时段乘数（乘数在计费时外挂，公式只表达模型基础价）。
// 本组件自治校验：保存前父组件调 validate()，未通过中止保存（后端保存链路同口径烟测兜底）

const props = defineProps<{
	modelValue: string
}>()

const emit = defineEmits<{
	'update:modelValue': [value: string]
}>()

const formulaText = ref(props.modelValue)
watch(
	() => props.modelValue,
	(v) => {
		formulaText.value = v
	},
)
watch(formulaText, (v) => {
	emit('update:modelValue', v)
	// 公式变更后上次校验结果失效
	validatedMark.value = ''
	validateError.value = ''
	validateResults.value = null
	usedVars.value = []
})

// 校验状态：validatedMark 记录「已通过校验的公式文本」，保存时公式与标记不一致须重新校验
const validating = ref(false)
const validatedMark = ref('')
const validateError = ref('')
const validateResults = ref<any[] | null>(null)
const usedVars = ref<string[]>([])

// 变量速查表（与后端 billing/formula 包 knownVars 同步维护）
const varDocs: Array<{ name: string; desc: string }> = [
	{ name: 'p', desc: '纯文本输入 token（公式单独计价的缓存/图片/音频已自动扣除）' },
	{ name: 'c', desc: '输出 token（引用 rt/img_o/ao 时自动扣除对应部分）' },
	{ name: 'ctx', desc: '完整输入上下文 token（含缓存，用于阶梯档位条件判断）' },
	{ name: 'cr', desc: '缓存读取（命中）token' },
	{ name: 'cc', desc: '缓存写入 5m token（Claude ephemeral_5m / OpenAI cache_write）' },
	{ name: 'cc1h', desc: '缓存写入 1h token（Claude ephemeral_1h）' },
	{ name: 'img / img_o', desc: '图片输入 / 输出 token' },
	{ name: 'ai / ao', desc: '音频输入 / 输出 token' },
	{ name: 'rt', desc: '推理输出 token（reasoning）' },
	{ name: 'image_count', desc: '图片张数（图片协议顶层 n / 异步任务请求张数）' },
	{ name: 'seconds', desc: '任务计费秒数（视频生成等时长类任务：预扣按请求时长，结算按官方实际秒数多退少补；同步请求恒 0）' },
]

// 已注册 usage facts key（校验接口下发 known_fact_keys；空 = 未请求过校验）
const knownFactKeys = ref<Record<string, string>>({})

// 示例模板（点击填充到输入框）
const formulaTemplates: Array<{ label: string; expr: string }> = [
	{ label: '纯 token 单价', expr: 'p * 5 + c * 20' },
	{ label: '缓存三件套', expr: 'p * 5 + cr * 1.25 + cc * 12.5 + cc1h * 25 + c * 50' },
	{ label: '推理 token 差价', expr: 'c * 10 + rt * 30' },
	{ label: '音频多模态', expr: 'p * 5 + img * 8 + ai * 10 + ao * 20 + c * 30' },
	{ label: '图片按张计费', expr: 'fixed(0.04) * image_count' },
	{ label: '长上下文档位', expr: 'ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40' },
	{ label: 'token + 按次附加费', expr: 'p * 5 + c * 20 + fixed(0.001)' },
	{ label: '语音按秒计费', expr: 'u("audio_seconds") * 400000' },
	{ label: '视频任务按秒', expr: 'fixed(0.4) * seconds' },
]

function applyTemplate(expr: string | number | boolean | Record<string, any> | Array<string | number | boolean | Record<string, any>>) {
	if (typeof expr !== 'string') return
	formulaText.value = expr
	Message.info('模板已填充，可编辑后点「校验并试算」')
}

async function doValidate(): Promise<boolean> {
	const src = formulaText.value.trim()
	if (!src) {
		validateError.value = '计费公式不能为空'
		Message.warning('计费公式不能为空')
		return false
	}
	validating.value = true
	try {
		const res: any = await request.post('/admin/models/pricing/validate-formula', { formula: src })
		const data = res.data?.data
		if (!data?.valid) {
			validateError.value = data?.error || '公式校验未通过'
			validateResults.value = data?.results || null
			usedVars.value = data?.vars || []
			Message.error(`公式校验未通过：${validateError.value}`)
			return false
		}
		validateError.value = ''
		validateResults.value = data?.results || []
		usedVars.value = data?.vars || []
		knownFactKeys.value = data?.known_fact_keys || {}
		validatedMark.value = src
		Message.success('公式校验通过')
		return true
	} catch {
		// 错误提示由 axios 拦截器统一处理
		return false
	} finally {
		validating.value = false
	}
}

// 保存前置校验入口：公式未变更且已通过 → 直接放行；否则重新校验
async function validate(): Promise<boolean> {
	if (formulaText.value.trim() && validatedMark.value === formulaText.value.trim()) {
		return true
	}
	return await doValidate()
}

defineExpose({ validate })

// 环境快照展示：只显示公式引用的变量（与后端返回一致，空环境不渲染行）
function envEntries(env: Record<string, number> | undefined): Array<[string, number]> {
	if (!env) return []
	return Object.entries(env).filter(([, v]) => Number(v) > 0)
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
		<div class="editor-section-header">
			<h3>表达式计费</h3>
			<span class="section-hint">
				系数单位 = {{ currencySymbol }} / 1M Token，金额 = 系数 ÷ 1M × 租户乘数 × 时段乘数；
				<code>fixed(x)</code> = 每次请求 x {{ currencySymbol }}；支持三目/比较/加减乘除与 min/max/abs/ceil/floor
			</span>
		</div>

		<AFormItem label="计费公式" required>
			<div class="formula-input-row">
				<ASelect size="small" placeholder="示例模板…" class="formula-template-select" @change="applyTemplate">
					<AOption v-for="tpl in formulaTemplates" :key="tpl.expr" :value="tpl.expr">{{ tpl.label }}</AOption>
				</ASelect>
				<AButton size="small" type="primary" :loading="validating" @click="doValidate">校验并试算</AButton>
			</div>
			<ATextarea
				v-model="formulaText"
				:auto-size="{ minRows: 3, maxRows: 8 }"
				placeholder="例如：ctx <= 128000 ? p * 5 + c * 20 : p * 10 + c * 40"
				class="formula-textarea"
			/>
		</AFormItem>

		<!-- 校验错误（行内红字，可直接展示后端中文报错） -->
		<div v-if="validateError" class="formula-error">
			<ATag color="red">校验未通过</ATag>
			<span>{{ validateError }}</span>
		</div>

		<!-- 引用变量 chips -->
		<div v-if="usedVars.length > 0" class="formula-vars">
			<span class="formula-vars-label">引用变量：</span>
			<ATag v-for="v in usedVars" :key="v" color="arcoblue" size="small">{{ v }}</ATag>
		</div>

		<!-- 试算结果表 -->
		<ATable
			v-if="validateResults && validateResults.length > 0"
			:data="validateResults"
			:pagination="false"
			size="small"
			class="formula-results"
		>
			<template #columns>
				<ATableColumn title="样例" data-index="name" :width="110" />
				<ATableColumn title="变量取值（非零）" :width="320">
					<template #cell="{ record }">
						<template v-if="envEntries(record.env).length > 0">
							<ATag v-for="[k, v] in envEntries(record.env)" :key="k" size="small" class="env-tag">
								{{ k }}={{ v.toLocaleString() }}
							</ATag>
						</template>
						<span v-else class="env-empty">（全零）</span>
					</template>
				</ATableColumn>
				<ATableColumn title="系数" :width="130">
					<template #cell="{ record }">
						<span class="mono">{{ record.factor.toLocaleString() }}</span>
					</template>
				</ATableColumn>
				<ATableColumn title="试算金额（未乘折扣）" :width="170">
					<template #cell="{ record }">
						<span class="mono">{{ fmtAmount(record.amount) }}</span>
					</template>
				</ATableColumn>
			</template>
		</ATable>

		<!-- 变量速查（默认折叠） -->
		<ACollapse :default-active-key="[]" class="formula-docs">
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

.editor-section-header {
	display: flex;
	align-items: baseline;
	gap: 10px;
}

.editor-section-header h3 {
	margin: 0;
	font-size: 15px;
	font-weight: 600;
}

.section-hint {
	font-size: 12px;
	color: var(--color-text-3);
}

.formula-input-row {
	display: flex;
	justify-content: flex-end;
	gap: 8px;
	margin-bottom: 8px;
}

.formula-template-select {
	width: 180px;
}

.formula-textarea :deep(textarea) {
	font-family: 'JetBrains Mono', Consolas, Monaco, monospace;
	font-size: 13px;
}

.formula-error {
	display: flex;
	align-items: center;
	gap: 8px;
	font-size: 13px;
	color: rgb(var(--red-6));
}

.formula-vars {
	display: flex;
	align-items: center;
	flex-wrap: wrap;
	gap: 4px;
}

.formula-vars-label {
	font-size: 12px;
	color: var(--color-text-3);
}

.formula-results {
	margin-top: 4px;
}

.env-tag {
	font-family: 'JetBrains Mono', Consolas, monospace;
	font-size: 12px;
}

.env-empty {
	font-size: 12px;
	color: var(--color-text-4);
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
