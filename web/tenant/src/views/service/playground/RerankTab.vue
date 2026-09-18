<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { NInput, NInputNumber } from 'naive-ui'
import { createPlaygroundApi } from '@/utils/playgroundApi'
import { calculateCost } from './calculateCost'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem {
	model_id: string
	model_name: string
	category: string
	billing_mode?: string | null
	per_request_price?: number | null
	input_price?: number | null
	output_price?: number | null
}
const props = defineProps<{ models: ModelItem[]; apiKey: string }>()

const sending = ref(false)
const errorMessage = ref('')
const selectedModel = ref(props.models[0]?.model_id || '')
// models 由父组件异步加载，setup 阶段可能为空；列表变化时若当前选中模型不在列表内
//（如切换 API Key 后模型范围变化）则自动重置为第一个
watch(
	() => props.models,
	models => {
		if (!selectedModel.value || !models.some(m => m.model_id === selectedModel.value)) {
			selectedModel.value = models[0]?.model_id || ''
		}
	},
)
const selectedModelItem = computed(() =>
	props.models.find(m => m.model_id === selectedModel.value),
)
const query = ref('')
const documentsText = ref('')
const topN = ref<number | undefined>(undefined)
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

interface RerankResult { index: number; relevance_score: number; document?: { text: string } }
const results = ref<RerankResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function rerank() {
	if (!query.value.trim() || !documentsText.value.trim() || !selectedModel.value) return
	const docs = documentsText.value.split('\n').filter(d => d.trim())
	if (docs.length === 0) return

	sending.value = true
	errorMessage.value = ''
	results.value = []

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.post('/v1/rerank', {
			model: selectedModel.value,
			query: query.value,
			documents: docs,
			top_n: topN.value || undefined,
		})

		const data = res.data
		results.value = data.results || []
		const usage = data.usage || {}
		tokenUsage.promptTokens = usage.prompt_tokens || 0
		tokenUsage.totalTokens = usage.total_tokens || 0
		tokenUsage.cost = calculateCost(selectedModelItem.value, usage) || ''
	} catch (e) {
		console.error(e)
		errorMessage.value = (e as any)?.message || '请求失败，请重试'
	} finally {
		sending.value = false
	}
}

function scoreColor(score: number) {
	if (score >= 0.8) return 'bg-emerald-500'
	if (score >= 0.5) return 'bg-amber-500'
	return 'bg-red-400'
}

// 空态里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => selectedModelItem.value?.model_name || selectedModel.value)

// 结果卡右上角的状态徽标（无任务且无错误时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	if (errorMessage.value) return { label: '失败', cls: 'badge-danger' }
	if (sending.value) return { label: '排序中', cls: 'badge-primary' }
	if (results.value.length) return { label: '已完成', cls: 'badge-success' }
	return null
})
</script>

<template>
	<!-- 与其余 Tab 同一套等高布局：左参数栏内部滚动，主操作常驻底部，右结果区撑满 -->
	<div class="flex flex-col lg:h-[calc(100vh-15rem)] lg:overflow-hidden">
		<div class="grid grid-cols-1 gap-4 lg:grid-cols-[22rem_minmax(0,1fr)] lg:grid-rows-[minmax(0,1fr)] lg:min-h-0 lg:flex-1">
			<!-- Left: 参数 -->
			<div class="flex lg:min-h-0">
				<div class="card flex w-full flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center gap-2 border-b border-gray-100 px-4 py-3">
						<Icon name="chart" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">重排序参数</h3>
					</div>

					<div class="flex-1 space-y-4 overflow-y-auto px-4 py-4">
						<!-- 模型 -->
						<div>
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">模型</label>
								<span class="text-[11px] text-gray-400">共 {{ models.length }} 个可用</span>
							</div>
							<BaseSelect v-model="selectedModel" :options="modelOptions" />
						</div>

						<!-- 输入 -->
						<div class="space-y-4 border-t border-gray-100 pt-4">
							<div>
								<label class="mb-2 block text-xs font-semibold text-gray-500">查询文本</label>
								<n-input v-model:value="query" type="textarea" :rows="2" placeholder="输入查询..." />
							</div>
							<div>
								<div class="mb-2 flex items-center justify-between">
									<label class="text-xs font-semibold text-gray-500">文档列表</label>
									<span class="text-[11px] text-gray-400">每行一个文档</span>
								</div>
								<n-input v-model:value="documentsText" type="textarea" :rows="6" placeholder="文档 1&#10;文档 2&#10;文档 3" />
							</div>
						</div>

						<!-- 输出参数 -->
						<div class="border-t border-gray-100 pt-4">
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">Top N</label>
								<span class="text-[11px] text-gray-400">留空返回全部</span>
							</div>
							<n-input-number v-model:value="topN" :min="1" class="w-full" placeholder="留空返回全部" />
						</div>
					</div>

					<!-- 主操作常驻卡片底部 -->
					<div class="shrink-0 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="sending || !query.trim() || !documentsText.trim()" @click="rerank">
							<Icon name="chart" size="sm" />
							{{ sending ? '排序中...' : '重排序' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Right: 结果 -->
			<div class="lg:min-h-0">
				<div class="card flex min-h-[420px] flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="chart" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">排序结果</h3>
						</div>
						<span v-if="resultStatus" class="badge" :class="resultStatus.cls">{{ resultStatus.label }}</span>
					</div>

					<div class="min-h-0 flex-1 overflow-y-auto p-5">
						<!-- 错误提示 -->
						<div
							v-if="errorMessage"
							class="mb-4 rounded-xl border border-red-200 bg-red-50 p-4"
						>
							<div class="flex items-center gap-2 text-red-700">
								<Icon name="xCircle" size="sm" />
								<span class="text-sm font-medium">排序失败</span>
							</div>
							<p class="mt-2 text-sm text-red-600">{{ errorMessage }}</p>
						</div>

						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="results.length === 0 && !sending && !errorMessage" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="chart" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待排序</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，填好查询与文档后点击「重排序」
								</template>
								<template v-else>请先在左侧选择一个重排模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								排序将真实调用模型并产生费用
							</p>
						</div>

						<!-- 排序中 -->
						<div v-if="sending" class="flex h-full min-h-[320px] flex-col items-center justify-center gap-3">
							<div class="spinner h-8 w-8 text-primary-600"></div>
							<span class="text-sm text-gray-500">重排序中...</span>
						</div>

						<!-- 结果 -->
						<div v-if="results.length > 0" class="space-y-3">
							<div v-for="r in results" :key="r.index" class="rounded-xl border border-gray-200 p-4">
								<div class="mb-2 flex items-center gap-3">
									<span class="badge badge-gray">#{{ r.index }}</span>
									<div class="h-2 flex-1 overflow-hidden rounded-full bg-gray-100">
										<div class="h-full rounded-full transition-all" :class="scoreColor(r.relevance_score)" :style="{ width: Math.max(r.relevance_score * 100, 2) + '%' }" />
									</div>
									<span class="text-sm font-medium text-gray-700">{{ (r.relevance_score * 100).toFixed(1) }}%</span>
								</div>
								<p v-if="r.document" class="text-sm whitespace-pre-wrap text-gray-600">{{ r.document.text }}</p>
							</div>

							<!-- 用量条：与其余 Tab 保持同一排版 -->
							<div class="flex items-center justify-between gap-3 border-t border-gray-100 pt-3 text-xs text-gray-500">
								<div class="flex items-center gap-4">
									<span class="flex items-center gap-1.5">Prompt <span class="font-medium text-gray-700">{{ tokenUsage.promptTokens }}</span></span>
									<span class="flex items-center gap-1.5">Total <span class="font-medium text-gray-700">{{ tokenUsage.totalTokens }}</span></span>
								</div>
								<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
