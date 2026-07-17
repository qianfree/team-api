<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
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
</script>

<template>
	<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
		<div class="lg:col-span-1">
			<div class="card sticky top-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">重排序参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>
					<div>
						<label class="input-label">查询文本</label>
						<textarea v-model="query" class="input" rows="2" placeholder="输入查询..." />
					</div>
					<div>
						<label class="input-label">文档列表（每行一个）</label>
						<textarea v-model="documentsText" class="input" rows="6" placeholder="文档 1&#10;文档 2&#10;文档 3" />
					</div>
					<div>
						<label class="input-label">Top N（可选）</label>
						<input v-model.number="topN" type="number" class="input" min="1" placeholder="留空返回全部" />
					</div>
					<button class="btn btn-primary w-full" :disabled="sending || !query.trim() || !documentsText.trim()" @click="rerank">
						{{ sending ? '排序中...' : '重排序' }}
					</button>
				</div>
			</div>
		</div>

		<div class="lg:col-span-3">
			<div class="card" style="min-height: 400px">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">排序结果</h3>
				</div>
				<div class="card-body">
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

					<div v-if="results.length === 0 && !sending && !errorMessage" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待排序</h3>
						<p class="empty-state-description">输入查询和文档并点击重排序</p>
					</div>
					<div v-if="sending" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">重排序中...</span>
					</div>
					<div v-if="results.length > 0" class="space-y-3">
						<div v-for="r in results" :key="r.index" class="rounded-xl border border-gray-200 p-4">
							<div class="flex items-center gap-3 mb-2">
								<span class="badge badge-gray">#{{ r.index }}</span>
								<div class="flex-1 h-2 bg-gray-100 rounded-full overflow-hidden">
									<div class="h-full rounded-full transition-all" :class="scoreColor(r.relevance_score)" :style="{ width: Math.max(r.relevance_score * 100, 2) + '%' }" />
								</div>
								<span class="text-sm font-medium text-gray-700">{{ (r.relevance_score * 100).toFixed(1) }}%</span>
							</div>
							<p v-if="r.document" class="text-sm text-gray-600 whitespace-pre-wrap">{{ r.document.text }}</p>
						</div>
						<div class="text-xs text-gray-500 flex items-center justify-between pt-3 border-t border-gray-100">
							<span>Tokens: {{ tokenUsage.promptTokens }} / {{ tokenUsage.totalTokens }}</span>
							<span v-if="tokenUsage.cost" class="font-medium text-amber-600">{{ tokenUsage.cost }}</span>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
