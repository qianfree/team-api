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
	errorMessage.value = ''
	embeddings.value = []

	try {
		const api = createPlaygroundApi(props.apiKey)
		const res = await api.post('/v1/embeddings', {
			model: selectedModel.value,
			input: inputText.value,
			dimensions: dimensions.value || undefined,
		})

		const data = res.data
		// 标准 OpenAI 格式: data.data[].embedding
		embeddings.value = (data.data || []).map((d: any) => ({
			index: d.index,
			embedding: d.embedding,
		}))
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

function formatEmbedding(values: number[], full: boolean) {
	if (full) return `[${values.map(v => v.toFixed(6)).join(', ')}]`
	const preview = values.slice(0, 8).map(v => v.toFixed(6)).join(', ')
	return `[${preview}, ... (${values.length} 维)]`
}

// 空态里展示的模型名（列表由父组件异步加载，可能暂时查不到）
const selectedModelName = computed(() => selectedModelItem.value?.model_name || selectedModel.value)

// 结果卡右上角的状态徽标（无任务且无错误时不渲染）
const resultStatus = computed<{ label: string; cls: string } | null>(() => {
	if (errorMessage.value) return { label: '失败', cls: 'badge-danger' }
	if (sending.value) return { label: '计算中', cls: 'badge-primary' }
	if (embeddings.value.length) return { label: '已完成', cls: 'badge-success' }
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
						<Icon name="cube" size="sm" class="text-primary-500" />
						<h3 class="text-sm font-semibold text-gray-900">嵌入参数</h3>
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

						<!-- 输入文本 -->
						<div class="border-t border-gray-100 pt-4">
							<label class="mb-2 block text-xs font-semibold text-gray-500">输入文本</label>
							<n-input v-model:value="inputText" type="textarea" :rows="5" placeholder="输入要嵌入的文本..." />
						</div>

						<!-- 输出参数 -->
						<div class="border-t border-gray-100 pt-4">
							<div class="mb-2 flex items-center justify-between">
								<label class="text-xs font-semibold text-gray-500">维度</label>
								<span class="text-[11px] text-gray-400">留空使用模型默认</span>
							</div>
							<n-input-number v-model:value="dimensions" class="w-full" placeholder="留空使用默认" />
						</div>
					</div>

					<!-- 主操作常驻卡片底部 -->
					<div class="shrink-0 border-t border-gray-100 px-4 py-3">
						<button class="btn btn-primary w-full" :disabled="sending || !inputText.trim()" @click="embed">
							<Icon name="cube" size="sm" />
							{{ sending ? '计算中...' : '计算嵌入' }}
						</button>
					</div>
				</div>
			</div>

			<!-- Right: 结果 -->
			<div class="lg:min-h-0">
				<div class="card flex min-h-[420px] flex-col overflow-hidden lg:h-full">
					<div class="flex shrink-0 items-center justify-between gap-3 border-b border-gray-100 px-5 py-3">
						<div class="flex items-center gap-2">
							<Icon name="cube" size="sm" class="text-primary-500" />
							<h3 class="text-sm font-semibold text-gray-900">嵌入结果</h3>
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
								<span class="text-sm font-medium">计算失败</span>
							</div>
							<p class="mt-2 text-sm text-red-600">{{ errorMessage }}</p>
						</div>

						<!-- 空态：图标磁贴 + 说明，给首屏一个视觉锚点 -->
						<div v-if="embeddings.length === 0 && !sending && !errorMessage" class="flex h-full min-h-[320px] flex-col items-center justify-center px-4 text-center">
							<div class="mb-5 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-cyan-400 via-primary-500 to-emerald-500 text-white shadow-glow">
								<Icon name="cube" size="lg" />
							</div>
							<h3 class="text-lg font-semibold text-gray-900">等待计算</h3>
							<p class="mt-1.5 max-w-sm text-sm text-gray-500">
								<template v-if="selectedModel">
									当前模型 <span class="font-medium text-gray-700">{{ selectedModelName }}</span>，输入文本后点击「计算嵌入」
								</template>
								<template v-else>请先在左侧选择一个嵌入模型</template>
							</p>
							<p class="mt-6 inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-[11px] text-amber-700">
								<Icon name="exclamationTriangle" size="xs" />
								计算将真实调用模型并产生费用
							</p>
						</div>

						<!-- 计算中 -->
						<div v-if="sending" class="flex h-full min-h-[320px] flex-col items-center justify-center gap-3">
							<div class="spinner h-8 w-8 text-primary-600"></div>
							<span class="text-sm text-gray-500">计算嵌入向量中...</span>
						</div>

						<!-- 结果 -->
						<div v-if="embeddings.length > 0" class="space-y-3">
							<div v-for="emb in embeddings" :key="emb.index" class="rounded-xl border border-gray-200 p-4">
								<div class="mb-2 flex items-center justify-between">
									<span class="badge badge-purple">Index: {{ emb.index }}</span>
									<span class="text-xs text-gray-500">{{ emb.embedding.length }} 维</span>
								</div>
								<code class="code-block block whitespace-pre-wrap break-all text-xs">
									{{ formatEmbedding(emb.embedding, showFullEmbedding === emb.index) }}
								</code>
								<button v-if="emb.embedding.length > 8" class="mt-2 text-xs text-primary-600 hover:underline" @click="showFullEmbedding = showFullEmbedding === emb.index ? null : emb.index">
									{{ showFullEmbedding === emb.index ? '收起' : '展开全部' }}
								</button>
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
