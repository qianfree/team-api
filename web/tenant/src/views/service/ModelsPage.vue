<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

interface ModelItem {
	id: number
	model_id: string
	model_name: string
	category: string
	max_context_tokens: number
	max_output_tokens: number
	description: string
	tags: string
	capabilities: string
	billing_mode: string | null
	per_request_price: number | null
	discount_ratio: number | null
	max_concurrency: number | null
}

const capabilityLabel: Record<string, string> = {
	vision: '图片识别',
	function_calling: '工具调用',
	parallel_function_calling: '并行工具调用',
	tool_choice: '工具选择',
	response_schema: '结构化输出',
	system_messages: '系统消息',
	prompt_caching: '提示词缓存',
	audio_input: '音频输入',
	audio_output: '音频输出',
	pdf_input: 'PDF 输入',
	embedding_image: '图像嵌入',
	reasoning: '深度思考',
	web_search: '联网搜索',
}

function parseCapabilities(raw: string): string[] {
	if (!raw) return []
	try {
		const obj = JSON.parse(raw)
		return Object.entries(obj)
			.filter(([, v]) => v === true)
			.map(([k]) => k)
	} catch {
		return []
	}
}

function parseTags(raw: string): string[] {
	if (!raw) return []
	try {
		const arr = JSON.parse(raw)
		return Array.isArray(arr) ? arr : []
	} catch {
		return []
	}
}

const copiedId = ref('')

function copyModelId(id: string) {
	navigator.clipboard.writeText(id)
	copiedId.value = id
	setTimeout(() => {
		if (copiedId.value === id) copiedId.value = ''
	}, 1500)
}

const models = ref<ModelItem[]>([])
const loading = ref(false)
const searchQuery = ref('')
const activeCategory = ref('')

const categories = [
	{ value: '', label: '全部' },
	{ value: 'chat', label: '对话' },
	{ value: 'embedding', label: '嵌入' },
	{ value: 'image', label: '图像' },
	{ value: 'audio', label: '语音' },
	{ value: 'rerank', label: '重排' },
]

const categoryLabel: Record<string, string> = {
	chat: '对话',
	embedding: '嵌入',
	image: '图像',
	audio: '语音',
	rerank: '重排',
}

const categoryColor: Record<string, string> = {
	chat: '#14b8a6',
	embedding: '#8b5cf6',
	image: '#f59e0b',
	audio: '#10b981',
	rerank: '#6b7280',
}

const filteredModels = computed(() => {
	let result = models.value
	if (activeCategory.value) {
		result = result.filter((m) => m.category === activeCategory.value)
	}
	if (searchQuery.value) {
		const q = searchQuery.value.toLowerCase()
		result = result.filter(
			(m) =>
				m.model_id.toLowerCase().includes(q) ||
				m.model_name.toLowerCase().includes(q)
		)
	}
	return result
})

const groupedModels = computed(() => {
	const groups: Record<string, ModelItem[]> = {}
	for (const m of filteredModels.value) {
		const cat = m.category || 'other'
		if (!groups[cat]) groups[cat] = []
		groups[cat].push(m)
	}
	return groups
})

function formatTokens(n: number): string {
	if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
	if (n >= 1000) return (n / 1000).toFixed(0) + 'K'
	return String(n)
}

async function fetchModels() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/models')
		const raw = res.data?.data; models.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
	} catch {
		// ignore
	} finally {
		loading.value = false
	}
}

onMounted(fetchModels)
</script>

<template>
	<div>
		<div class="page-header">
			<h1 class="page-title">可用模型</h1>
			<p class="page-description">查看当前组织可使用的 AI 模型及其配置信息</p>
		</div>

		<!-- Filters -->
		<div class="card mb-6">
			<div class="flex flex-col gap-4 px-6 py-4 sm:flex-row sm:items-center sm:justify-between">
				<div class="tabs">
					<button
						v-for="cat in categories"
						:key="cat.value"
						class="tab"
						:class="{ 'tab-active': activeCategory === cat.value }"
						@click="activeCategory = cat.value"
					>
						{{ cat.label }}
						<span v-if="cat.value" class="ml-1 text-xs text-gray-400">
							({{ models.filter((m) => m.category === cat.value).length }})
						</span>
						<span v-else class="ml-1 text-xs text-gray-400">
							({{ models.length }})
						</span>
					</button>
				</div>
				<div class="relative w-full sm:w-64">
					<Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
					<input v-model="searchQuery" type="text" class="input pl-9" placeholder="搜索模型..." />
				</div>
			</div>
		</div>

		<!-- Loading -->
		<div v-if="loading" class="flex items-center justify-center py-12">
			<div class="spinner h-6 w-6 text-primary-500"></div>
		</div>

		<!-- Empty -->
		<div v-else-if="models.length === 0" class="card">
			<div class="empty-state">
				<Icon name="cube" size="xl" class="empty-state-icon" />
				<h3 class="empty-state-title">暂无可用模型</h3>
				<p class="empty-state-description">请联系管理员为您的组织分配模型</p>
			</div>
		</div>

		<!-- No results after filter -->
		<div v-else-if="filteredModels.length === 0" class="card">
			<div class="empty-state">
				<Icon name="search" size="xl" class="empty-state-icon" />
				<h3 class="empty-state-title">未找到匹配的模型</h3>
				<p class="empty-state-description">尝试更换筛选条件或搜索关键词</p>
			</div>
		</div>

		<!-- Model list grouped by category -->
		<div v-else class="space-y-6">
			<div v-for="(items, cat) in groupedModels" :key="cat">
				<h2 class="mb-3 text-sm font-semibold text-gray-500 uppercase tracking-wider">
					{{ categoryLabel[cat] || cat }}
					<span class="text-gray-300">({{ items.length }})</span>
				</h2>
				<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
					<div
						v-for="m in items"
						:key="m.id"
						class="card card-hover p-5 flex flex-col relative overflow-hidden"
					>
						<!-- Discount ribbon (top-right corner) -->
						<div
							v-if="m.discount_ratio && m.discount_ratio < 1"
							class="absolute top-3 -right-6 w-24 h-5 flex items-center justify-center"
							style="transform: rotate(45deg); background: linear-gradient(135deg, #f43f5e, #fb923c); box-shadow: 0 2px 8px rgba(244,63,94,0.35);"
						>
							<span class="text-white font-bold text-[10px] tracking-wide">
								{{ '-' + ((1 - m.discount_ratio) * 100).toFixed(0) + '%' }}
							</span>
						</div>

						<!-- Section 1: Category + Name -->
						<div class="mb-3" :class="{ 'pr-14': m.discount_ratio && m.discount_ratio < 1 }">
							<div class="flex items-center gap-2">
								<span
									class="shrink-0 px-2.5 py-0.5 text-[10px] font-medium text-white rounded-full"
									:style="{ background: categoryColor[m.category] || '#6b7280' }"
								>
									{{ categoryLabel[m.category] || m.category }}
								</span>
								<h3 class="text-sm font-semibold text-gray-900 truncate">{{ m.model_name || m.model_id }}</h3>
							</div>
							<div class="flex items-center gap-1 mt-0.5">
								<p class="text-xs text-gray-400 font-mono truncate">{{ m.model_id }}</p>
								<button
									class="shrink-0 p-0.5 rounded text-gray-300 hover:text-primary-500 transition-colors duration-150"
									title="复制模型编码"
									@click="copyModelId(m.model_id)"
								>
									<Icon v-if="copiedId !== m.model_id" name="copy" size="xs" />
									<Icon v-else name="check" size="xs" class="text-emerald-500" />
								</button>
							</div>
						</div>

						<!-- Section 2: Description (fixed height, always reserved) -->
						<div class="mb-3 h-8">
							<p v-if="m.description" class="text-xs text-gray-500 line-clamp-2">{{ m.description }}</p>
							<span v-else class="text-xs text-gray-300">暂无描述</span>
						</div>

						<!-- Section 3: Token info + Tags -->
						<div class="flex items-center justify-between gap-2 text-xs text-gray-500 flex-1">
							<div class="flex flex-wrap gap-3">
								<span class="flex items-center gap-1">
									<Icon name="document" size="xs" class="text-gray-300" />
									{{ m.max_context_tokens ? formatTokens(m.max_context_tokens) : '-' }}
								</span>
								<span class="flex items-center gap-1">
									<Icon name="arrowUp" size="xs" class="text-gray-300" />
									{{ m.max_output_tokens ? formatTokens(m.max_output_tokens) : '-' }}
								</span>
								<span class="flex items-center gap-1">
									<Icon name="chart" size="xs" class="text-gray-300" />
									{{ m.max_concurrency ?? '-' }}
								</span>
							</div>
							<div v-if="parseTags(m.tags).length" class="flex flex-wrap gap-1 justify-end">
								<span
									v-for="tag in parseTags(m.tags)"
									:key="tag"
									class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-medium text-gray-500"
								>
									{{ tag }}
								</span>
							</div>
						</div>

						<!-- Section 4: Capabilities (bottom, fixed height) -->
						<div class="mt-3 pt-3 border-t border-gray-100 min-h-[28px]">
							<div v-if="parseCapabilities(m.capabilities).length" class="flex flex-wrap gap-1">
								<span
									v-for="cap in parseCapabilities(m.capabilities)"
									:key="cap"
									class="inline-flex items-center rounded-md bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600"
								>
									{{ capabilityLabel[cap] || cap }}
								</span>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
