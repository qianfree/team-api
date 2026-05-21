<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import request from '@/utils/request'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../../components/common/BaseSelect.vue'

interface ModelItem { model_id: string; model_name: string; category: string }
const props = defineProps<{ models: ModelItem[] }>()

const sending = ref(false)
const selectedModel = ref(props.models[0]?.model_id || '')
const inputText = ref('')
const dimensions = ref<number | undefined>(undefined)
const modelOptions = computed(() => props.models.map(m => ({ value: m.model_id, label: m.model_name || m.model_id })))

interface EmbeddingResult { index: number; embedding: number[] }
const embeddings = ref<EmbeddingResult[]>([])
const showFullEmbedding = ref<number | null>(null)
const tokenUsage = reactive({ promptTokens: 0, totalTokens: 0, cost: '' })

async function embed() {
	if (!inputText.value.trim() || !selectedModel.value) return
	sending.value = true
	embeddings.value = []

	try {
		const res = await request.post('/tenant/playground/embedding', {
			model: selectedModel.value,
			input: inputText.value,
			dimensions: dimensions.value || undefined,
		})
		if (res.data?.code === 0) {
			const data = res.data.data
			embeddings.value = data.embeddings || []
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

function formatEmbedding(values: number[], full: boolean) {
	if (full) return `[${values.map(v => v.toFixed(6)).join(', ')}]`
	const preview = values.slice(0, 8).map(v => v.toFixed(6)).join(', ')
	return `[${preview}, ... (${values.length} 维)]`
}
</script>

<template>
	<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
		<div class="lg:col-span-1">
			<div class="card sticky top-6">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">嵌入参数</h3>
				</div>
				<div class="card-body space-y-4">
					<div>
						<label class="input-label">模型</label>
						<BaseSelect v-model="selectedModel" :options="modelOptions" />
					</div>
					<div>
						<label class="input-label">输入文本</label>
						<textarea v-model="inputText" class="input" rows="4" placeholder="输入要嵌入的文本..." />
					</div>
					<div>
						<label class="input-label">维度（可选）</label>
						<input v-model.number="dimensions" type="number" class="input" placeholder="留空使用默认" />
					</div>
					<button class="btn btn-primary w-full" :disabled="sending || !inputText.trim()" @click="embed">
						{{ sending ? '计算中...' : '计算嵌入' }}
					</button>
				</div>
			</div>
		</div>

		<div class="lg:col-span-3">
			<div class="card" style="min-height: 400px">
				<div class="card-header">
					<h3 class="text-sm font-semibold text-gray-900">嵌入结果</h3>
				</div>
				<div class="card-body">
					<div v-if="embeddings.length === 0 && !sending" class="empty-state">
						<div class="empty-state-icon"><Icon name="bookOpen" size="xl" /></div>
						<h3 class="empty-state-title">等待计算</h3>
						<p class="empty-state-description">输入文本并点击计算</p>
					</div>
					<div v-if="sending" class="flex items-center justify-center py-12">
						<div class="spinner h-8 w-8 text-primary-600"></div>
						<span class="ml-3 text-sm text-gray-500">计算嵌入向量中...</span>
					</div>
					<div v-if="embeddings.length > 0" class="space-y-3">
						<div v-for="emb in embeddings" :key="emb.index" class="rounded-xl border border-gray-200 p-4">
							<div class="flex items-center justify-between mb-2">
								<span class="badge badge-purple">Index: {{ emb.index }}</span>
								<span class="text-xs text-gray-500">{{ emb.embedding.length }} 维</span>
							</div>
							<code class="code-block text-xs block whitespace-pre-wrap break-all">
								{{ formatEmbedding(emb.embedding, showFullEmbedding === emb.index) }}
							</code>
							<button v-if="emb.embedding.length > 8" class="text-xs text-primary-600 mt-2 hover:underline" @click="showFullEmbedding = showFullEmbedding === emb.index ? null : emb.index">
								{{ showFullEmbedding === emb.index ? '收起' : '展开全部' }}
							</button>
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
