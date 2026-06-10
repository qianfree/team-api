<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { Message } from '@arco-design/web-vue'
import request from '@/utils/request'

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

const billingModeOptions = [
	{ label: '按量计费 (Token)', value: 'token' },
	{ label: '按次计费', value: 'per_request' },
	{ label: '阶梯计费', value: 'tiered' },
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
	}
}

function resetEditorDefaults() {
	editorBillingMode.value = 'token'
	editorItems.length = 0
	editorItems.push(createEmptyItem('token'))
}

// ============================================================
// Official Pricing Fetch
// ============================================================
const officialLoading = ref(false)
const officialData = ref<any>(null)
const officialError = ref('')

function resetOfficialData() {
	officialData.value = null
	officialError.value = ''
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
		editorItems[0].input_price = pricing.input_price
		editorItems[0].output_price = pricing.output_price
		editorItems[0].cache_read_price = pricing.cache_read_price
		editorItems[0].cache_creation_price = pricing.cache_creation_price
	}
	if (pricing.billing_mode && pricing.billing_mode !== editorBillingMode.value) {
		editorBillingMode.value = pricing.billing_mode
	}
	Message.success('已填入官方参考定价')
}

// ============================================================
// Load Pricing Data
// ============================================================
async function loadPricing() {
	if (!props.modelId) return
	editorItems.length = 0
	editorBillingMode.value = 'token'
	resetOfficialData()
	editorLoading.value = true

	try {
		const res: any = await request.get(`/admin/models/${props.modelId}/pricing`)
		const list: any[] = res.data?.data?.list || []
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
				})
			}
		} else {
			resetEditorDefaults()
		}
	} catch {
		resetEditorDefaults()
	} finally {
		editorLoading.value = false
	}
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
		const items = editorItems.map((item, index) => ({
			billing_mode: editorBillingMode.value,
			min_tokens: editorBillingMode.value === 'tiered' ? item.min_tokens : 0,
			max_tokens: editorBillingMode.value === 'tiered' ? (index < editorItems.length - 1 ? item.max_tokens : null) : null,
			input_price: item.input_price,
			output_price: item.output_price,
			per_request_price: editorBillingMode.value === 'per_request' ? (item.per_request_price ?? null) : null,
			cache_read_price: item.cache_read_price,
			cache_creation_price: item.cache_creation_price,
		}))
		await request.put(`/admin/models/${props.modelId}/pricing`, { items })
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
	<ADrawer
		:visible="visible"
		:width="580"
		:title="modelId ? `定价设置 - ${modelIdStr}` : '定价设置'"
		:mask-closable="false"
		:footer="true"
		@cancel="emit('update:visible', false)"
	>
		<template #footer>
			<AButton @click="emit('update:visible', false)">取消</AButton>
			<AButton type="primary" :loading="editorSaving" @click="savePricing">保存定价</AButton>
		</template>
		<ASpin :loading="editorLoading" style="width: 100%">
		<AForm :model="{}" layout="vertical">
			<div class="pricing-editor">
				<!-- Official Pricing Reference -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>官方参考定价</h3>
						<span class="section-hint">数据来源: LiteLLM + models.dev</span>
					</div>
					<AButton
						type="outline"
						size="small"
						:loading="officialLoading"
						@click="fetchOfficialPricing"
					>拉取官方定价</AButton>

					<div v-if="officialData" class="official-pricing-card mt-3">
						<div v-for="src in officialData.sources" :key="src.source" class="official-source-block">
							<div class="official-source-header">
								<ATag size="small" :color="src.source === 'litellm' ? 'arcoblue' : 'green'">{{ src.source === 'litellm' ? 'LiteLLM' : 'models.dev' }}</ATag>
								<template v-if="src.found && src.pricing">
									<ATag v-if="src.provider" size="small" color="orangered">{{ src.provider }}</ATag>
									<ATag v-if="src.mode" size="small">{{ src.mode }}</ATag>
									<span v-if="src.max_context_tokens" class="official-meta-text">上下文: {{ (src.max_context_tokens / 1000).toFixed(0) }}K</span>
									<span v-if="src.max_output_tokens" class="official-meta-text">输出上限: {{ (src.max_output_tokens / 1000).toFixed(0) }}K</span>
								</template>
							</div>
							<template v-if="src.found && src.pricing">
								<div class="official-prices mt-2">
									<div class="official-price-row">
										<span class="official-price-label">输入价格</span>
										<span class="official-price-value">${{ src.pricing.input_price.toFixed(2) }} / 1M</span>
									</div>
									<div class="official-price-row">
										<span class="official-price-label">输出价格</span>
										<span class="official-price-value">${{ src.pricing.output_price.toFixed(2) }} / 1M</span>
									</div>
									<div class="official-price-row">
										<span class="official-price-label">缓存读取</span>
										<span class="official-price-value">{{ src.pricing.cache_read_price ? '$' + src.pricing.cache_read_price.toFixed(2) + ' / 1M' : '—' }}</span>
									</div>
									<div class="official-price-row">
										<span class="official-price-label">缓存创建</span>
										<span class="official-price-value">{{ src.pricing.cache_creation_price ? '$' + src.pricing.cache_creation_price.toFixed(2) + ' / 1M' : '—' }}</span>
									</div>
								</div>
								<AButton type="primary" size="small" class="mt-2" @click="applyOfficialPricing(src.pricing)">应用此价格</AButton>
							</template>
							<template v-else>
								<div v-if="src.error" class="official-not-found" style="color: rgb(var(--danger-6))">获取远程数据失败: {{ src.error }}</div>
								<div v-else class="official-not-found">未找到定价数据</div>
							</template>
						</div>
					</div>
					<div v-if="officialError" class="official-error mt-2">{{ officialError }}</div>
				</div>

				<!-- Billing Mode -->
				<div class="editor-section">
					<div class="editor-section-header">
						<h3>计费模式</h3>
					</div>
					<ARadioGroup v-model="editorBillingMode" type="button">
						<ARadio v-for="opt in billingModeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</ARadio>
					</ARadioGroup>
				</div>

				<!-- Per-request Mode -->
				<template v-if="editorBillingMode === 'per_request' && editorItems.length > 0">
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>按次定价</h3>
						</div>
						<div class="form-row">
							<AFormItem label="按次单价" class="flex-1">
								<AInputNumber
									v-model="editorItems[0].per_request_price"
									:min="0"
									:precision="4"
									placeholder="每次调用价格"
									class="w-full"
								>
									<template #suffix>$ / 次</template>
								</AInputNumber>
							</AFormItem>
						</div>
					</div>
				</template>

				<!-- Token Mode -->
				<template v-else-if="editorBillingMode === 'token' && editorItems.length > 0">
					<div class="editor-section">
						<div class="editor-section-header">
							<h3>按量定价</h3>
							<span class="section-hint">价格单位为 $ / 1M Token</span>
						</div>
						<div class="form-row form-row--two">
							<AFormItem label="输入价格" class="flex-1">
								<AInputNumber
									v-model="editorItems[0].input_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>$ / 1M</template>
								</AInputNumber>
							</AFormItem>
							<AFormItem label="输出价格" class="flex-1">
								<AInputNumber
									v-model="editorItems[0].output_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>$ / 1M</template>
								</AInputNumber>
							</AFormItem>
						</div>
						<div class="form-row form-row--two mt-3">
							<AFormItem label="缓存读取价格" class="flex-1">
								<AInputNumber
									v-model="editorItems[0].cache_read_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>$ / 1M</template>
								</AInputNumber>
							</AFormItem>
							<AFormItem label="缓存创建价格" class="flex-1">
								<AInputNumber
									v-model="editorItems[0].cache_creation_price"
									:min="0"
									:precision="4"
									placeholder="0"
									class="w-full"
								>
									<template #suffix>$ / 1M</template>
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
							<div class="form-row form-row--two">
								<AFormItem label="起始 Token" class="flex-1">
									<AInputNumber
										v-model="tier.min_tokens"
										:min="0"
										:step="1000"
										placeholder="0"
										class="w-full"
									/>
								</AFormItem>
								<AFormItem label="结束 Token" class="flex-1">
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
							</div>
							<div class="form-row form-row--two mt-2">
								<AFormItem label="输入价格" class="flex-1">
									<AInputNumber
										v-model="tier.input_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>$/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="输出价格" class="flex-1">
									<AInputNumber
										v-model="tier.output_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>$/1M</template>
									</AInputNumber>
								</AFormItem>
							</div>
							<div class="form-row form-row--two mt-2">
								<AFormItem label="缓存读取价格" class="flex-1">
									<AInputNumber
										v-model="tier.cache_read_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>$/1M</template>
									</AInputNumber>
								</AFormItem>
								<AFormItem label="缓存创建价格" class="flex-1">
									<AInputNumber
										v-model="tier.cache_creation_price"
										:min="0"
										:precision="4"
										class="w-full"
									>
										<template #suffix>$/1M</template>
									</AInputNumber>
								</AFormItem>
							</div>
						</div>
						<AButton type="dashed" long @click="addTier" class="mt-3">+ 添加梯度</AButton>
					</div>
				</template>
			</div>
		</AForm>
		</ASpin>
	</ADrawer>
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

.form-row {
	display: flex;
	gap: 16px;
}

.form-row--two {
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

.official-pricing-card {
	padding: 12px 16px;
	background: var(--color-fill-1);
	border-radius: 8px;
	border: 1px solid var(--ta-border-light);
}

.official-source-block {
	padding: 10px 0;
	border-bottom: 1px dashed var(--ta-border-light);
}

.official-source-block:last-child {
	border-bottom: none;
	padding-bottom: 0;
}

.official-source-block:first-child {
	padding-top: 0;
}

.official-source-header {
	display: flex;
	align-items: center;
	gap: 6px;
	flex-wrap: wrap;
}

.official-meta-text {
	font-size: 12px;
	color: var(--ta-text-tertiary);
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
	font-size: 13px;
	color: var(--ta-text-tertiary);
}

.official-error {
	font-size: 13px;
	color: rgb(var(--danger-6));
}
</style>
