<script setup lang="ts">
import { ref, reactive } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
const props = defineProps<{ models: ModelItem[] }>()

const sending = ref(false)
const selectedModel = ref(props.models[0]?.model_id || '')
const query = ref('')
const documentsText = ref('')
const topN = ref<number | undefined>(undefined)

interface RerankResult { index: number; relevance_score: number; document?: { text: string } }
const results = ref<RerankResult[]>([])
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function rerank() {
	if (!query.value.trim() || !documentsText.value.trim() || !selectedModel.value) return
	const docs = documentsText.value.split('\n').filter(d => d.trim())
	if (docs.length === 0) return

	sending.value = true
	results.value = []

	try {
		const res = await request.post('/tenant/playground/rerank', {
			model: selectedModel.value,
			query: query.value,
			documents: docs,
			top_n: topN.value || undefined,
		})
		if (res.data?.code === 0) {
			const data = res.data.data
			results.value = data.results || []
			tokenUsage.promptTokens = data.prompt_tokens || 0
			tokenUsage.totalTokens = data.total_tokens || 0
			tokenUsage.cost = data.estimated_cost || ''
		}
	} catch (e) {
		console.error(e)
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
						<select v-model="selectedModel" class="input">
							<option v-for="m in models" :key="m.model_id" :value="m.model_id">{{ m.model_name || m.model_id }}</option>
						</select>
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
					<div v-if="results.length === 0 && !sending" class="empty-state">
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
