<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'
import { currencySymbol } from '@/composables/useCurrency'

// MiniMax 素材计费方案编辑器（scheme = custom:minimax-material，契约见 pricingSchemeEditors.ts）：
//   费用 = 输出生成（per_second 矩阵，存 pricing JSONB prices）
//        + 输入视频（按输入时长 × 生成分辨率单价，复用输出生成矩阵，无需单独配置）
//        + 输入图片（免费额度 + 单价，存 scheme_config.image）
//   输入音频免费（无需配置）。时段定价 / 参数倍率不适用于素材计费，保存时恒清除。
//   展示字段（折扣标签 / 价格调整说明 / 价格说明）与通用表单同源，随保存全量替换提交
//   （后端空串=清除，缺省不传同样清空，故必须显式回传）。
// 金额单位均为本位币（bil 层存储原值，不折算）。

const props = defineProps<{
	modelId: number
	schemeConfig?: any
}>()

interface PriceRow {
	spec: string
	price: number
}

// 输出生成矩阵（规格 → 每秒单价）
const outputRows = reactive<PriceRow[]>([])
// 输入图片素材
const imageFreeCount = ref<number>(5)
const imagePrice = ref<number>(0)
// 展示字段（挂锚点行：对外营销文案 + 内部备注，与通用定价表单同源）
const discountLabel = ref('')
const priceChangeNote = ref('')
const priceNote = ref('')

const outputSpecPresets = ['480P', '768P', '2K', '*']

function addRow(rows: PriceRow[], spec = '') {
	rows.push({ spec, price: 0 })
}

function removeRow(rows: PriceRow[], index: number) {
	rows.splice(index, 1)
}

// buildMatrix 行数组 → 单价矩阵；重复规格 / 负价校验失败返回 null（requirePositive 要求至少一档正价）
function buildMatrix(rows: PriceRow[], requirePositive: boolean): Record<string, number> | null {
	const map: Record<string, number> = {}
	let hasPositive = false
	for (const row of rows) {
		const spec = row.spec.trim()
		if (!spec) continue
		if (map[spec] !== undefined) {
			Message.warning(`规格「${spec}」重复`)
			return null
		}
		if (row.price < 0) {
			Message.warning(`规格「${spec}」单价不能为负`)
			return null
		}
		map[spec] = row.price
		if (row.price > 0) hasPositive = true
	}
	if (requirePositive && !hasPositive) {
		Message.warning('输出生成至少配置一档正价（建议额外配置 * 兜底价）')
		return null
	}
	return map
}

onMounted(async () => {
	try {
		const res: any = await request.get(`/admin/models/${props.modelId}/pricing`)
		const data = res.data?.data
		// 输出生成矩阵（per_second 锚点行）
		outputRows.length = 0
		const prices = data?.list?.[0]?.per_second_prices || {}
		for (const [spec, price] of Object.entries(prices)) {
			outputRows.push({ spec, price: Number(price) || 0 })
		}
		// 素材配置（scheme_config）
		const cfg = data?.scheme_config || props.schemeConfig || {}
		imageFreeCount.value = cfg.image?.free_count ?? 5
		imagePrice.value = Number(cfg.image?.price_per_unit) || 0
		// 展示字段（锚点行直存列，与通用表单同源）
		discountLabel.value = data?.discount_label || ''
		priceChangeNote.value = data?.price_change_note || ''
		priceNote.value = data?.price_note || ''
	} catch {
		// error handled by interceptor；回显留空可手工填写
	}
})

async function save(): Promise<boolean> {
	// 输出生成矩阵：至少一档正价（该矩阵同时用于输出生成与输入视频按秒计费）
	const outputMap = buildMatrix(outputRows, true)
	if (!outputMap) return false
	// 图片参数
	if (imageFreeCount.value < 0) {
		Message.warning('输入图片免费张数不能为负')
		return false
	}
	if (imagePrice.value < 0) {
		Message.warning('输入图片单价不能为负')
		return false
	}
	// 时段 / 参数倍率不适用于素材计费，恒传空数组清除（全量替换语义）

	// scheme_config 仅含图片配置（0 = 图片免费；输入视频按输出生成矩阵计费，无需配置）
	const schemeConfig: any = {}
	if (imagePrice.value > 0) {
		schemeConfig.image = { free_count: imageFreeCount.value, price_per_unit: imagePrice.value }
	}

	await request.put(`/admin/models/${props.modelId}/pricing`, {
		items: [
			{
				// special = 特殊计费模式（与 per_second 平级）：声明该模型走 scheme 方案计费，
				// 定价形态仍是按秒矩阵（输出生成组件）+ scheme_config 素材单价
				billing_mode: 'special',
				min_tokens: 0,
				max_tokens: null,
				input_price: 0,
				output_price: 0,
				per_request_price: null,
				cache_read_price: 0,
				cache_creation_price: 0,
				per_second_prices: outputMap,
			},
		],
		scheme: 'custom:minimax-material',
		scheme_config: schemeConfig,
		time_segments: [],
		param_multipliers: [],
		// 展示字段全量替换提交（后端空串=清除；缺省不传同样清空，必须显式回传）
		price_note: priceNote.value.trim(),
		discount_label: discountLabel.value.trim(),
		price_change_note: priceChangeNote.value.trim(),
	})
	return true
}

defineExpose({ save })
</script>

<template>
	<div class="mm-pricing-editor">
		<div class="editor-section">
			<div class="section-header">
				<h3>输出生成计费</h3>
				<span class="section-hint">生成时长 × 生成分辨率每秒单价（音频输入免费，输入图片/视频见下方素材计费）</span>
			</div>
			<div class="matrix-rows">
				<div v-for="(row, i) in outputRows" :key="i" class="matrix-row">
					<AInput v-model="row.spec" placeholder="规格（如 768P / 2K / *）" size="small" class="spec-input" />
					<AInputNumber v-model="row.price" :min="0" :step="0.01" size="small" class="price-input">
						<template #suffix>{{ currencySymbol }} / 秒</template>
					</AInputNumber>
					<AButton size="small" status="danger" type="text" @click="removeRow(outputRows, i)">删除</AButton>
				</div>
			</div>
			<div class="preset-row">
				<ATag
					v-for="spec in outputSpecPresets"
					:key="spec"
					class="preset-tag"
					@click="addRow(outputRows, spec)"
				>
					+ {{ spec }}
				</ATag>
			</div>
		</div>

		<div class="editor-section">
			<div class="section-header">
				<h3>输入素材计费</h3>
				<span class="section-hint">音频免费；图片按张（含免费额度）；输入视频自动按输出生成矩阵单价 × 输入时长计费（结算以官方 usage 为准，无需单独配置）</span>
			</div>
			<div class="image-config">
				<div class="field-item">
					<span class="field-label">图片免费张数（每次请求）</span>
					<AInputNumber v-model="imageFreeCount" :min="0" :precision="0" size="small" class="field-input">
						<template #suffix>张</template>
					</AInputNumber>
				</div>
				<div class="field-item">
					<span class="field-label">超出部分单价</span>
					<AInputNumber v-model="imagePrice" :min="0" :step="0.01" size="small" class="field-input">
						<template #suffix>{{ currencySymbol }} / 张</template>
					</AInputNumber>
				</div>
			</div>
		</div>

		<div class="editor-section">
			<div class="section-header">
				<h3>展示信息</h3>
				<span class="section-hint">折扣标签与调整说明会展示给租户；价格说明仅管理后台可见</span>
			</div>
			<div class="display-config">
				<div class="field-item">
					<span class="field-label">折扣标签（对外展示）</span>
					<AInput v-model="discountLabel" :maxlength="50" placeholder="如：7折起、限时5折" allow-clear size="small" class="field-input-flex" />
				</div>
				<div class="field-item">
					<span class="field-label">价格调整说明（对外展示）</span>
					<AInput v-model="priceChangeNote" :maxlength="200" placeholder="如：9月1日起价格下调 20%" allow-clear size="small" class="field-input-flex" />
				</div>
				<div class="field-item field-item-top">
					<span class="field-label">价格说明（仅内部可见）</span>
					<ATextarea
						v-model="priceNote"
						:max-length="500"
						:auto-size="{ minRows: 2, maxRows: 4 }"
						placeholder="调价背景、渠道成本等内部备注，不对外展示"
						class="field-input-flex"
					/>
				</div>
			</div>
		</div>
	</div>
</template>

<style scoped>
.mm-pricing-editor {
	display: flex;
	flex-direction: column;
	gap: 20px;
}

.editor-section {
	display: flex;
	flex-direction: column;
	gap: 10px;
}

.section-header {
	display: flex;
	align-items: baseline;
	gap: 10px;
}

.section-header h3 {
	margin: 0;
	font-size: 14px;
}

.section-hint {
	font-size: 12px;
	color: var(--color-text-3);
}

.subsection-label {
	font-size: 13px;
	color: var(--color-text-2);
	margin-top: 4px;
}

.matrix-rows {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.matrix-row {
	display: flex;
	align-items: center;
	gap: 8px;
}

.spec-input {
	width: 220px;
}

.price-input {
	flex: 1;
}

.preset-row {
	display: flex;
	gap: 6px;
}

.preset-tag {
	cursor: pointer;
}

.image-config {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.display-config {
	display: flex;
	flex-direction: column;
	gap: 8px;
}

.field-input-flex {
	flex: 1;
	min-width: 0;
}

/* 多行文本行：标签顶对齐，避免 textarea 垂直居中错位 */
.field-item-top {
	align-items: flex-start;
}

.field-item-top .field-label {
	margin-top: 4px;
}

.field-item {
	display: flex;
	align-items: center;
	gap: 12px;
}

.field-label {
	width: 180px;
	font-size: 13px;
	color: var(--color-text-2);
}

.field-input {
	width: 200px;
}

@media (max-width: 600px) {
	.matrix-row {
		flex-wrap: wrap;
	}

	.spec-input {
		width: 100%;
	}

	.field-label {
		width: 120px;
	}
}
</style>
