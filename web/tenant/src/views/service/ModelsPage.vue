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
	billing_mode: string | null
	per_request_price: number | null
	discount_ratio: number | null
	max_concurrency: number | null
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

const categoryBadgeClass: Record<string, string> = {
	chat: 'badge-primary',
	embedding: 'badge-purple',
	image: 'badge-warning',
	audio: 'badge-success',
	rerank: 'badge-gray',
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
				<!-- Category tabs -->
				<div class="tabs">
					<button
						v-for="cat in categories"
						:key="cat.value"
						class="tab"
						:class="{ 'tab-active': activeCategory === cat.value }"
						@click="activeCategory = cat.value"
					>
						{{ cat.label }}
						<span
							v-if="cat.value"
							class="ml-1 text-xs text-gray-400"
						>
							({{ models.filter((m) => m.category === cat.value).length }})
						</span>
						<span v-else class="ml-1 text-xs text-gray-400">
							({{ models.length }})
						</span>
					</button>
				</div>

				<!-- Search -->
				<div class="relative w-full sm:w-64">
					<Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
					<input
						v-model="searchQuery"
						type="text"
						class="input pl-9"
						placeholder="搜索模型..."
					/>
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
						class="card card-hover p-5"
					>
						<div class="flex items-start justify-between gap-2 mb-3">
							<div class="min-w-0">
								<h3 class="text-sm font-semibold text-gray-900 truncate">{{ m.model_name || m.model_id }}</h3>
								<p class="text-xs text-gray-400 font-mono truncate mt-0.5">{{ m.model_id }}</p>
							</div>
							<span class="badge shrink-0" :class="categoryBadgeClass[m.category] || 'badge-gray'">
								{{ categoryLabel[m.category] || m.category }}
							</span>
						</div>

						<p v-if="m.description" class="text-xs text-gray-500 mb-3 line-clamp-2">{{ m.description }}</p>

						<div class="flex flex-wrap gap-3 text-xs text-gray-500">
							<span v-if="m.max_context_tokens" class="flex items-center gap-1">
								<Icon name="document" size="xs" class="text-gray-300" />
								上下文 {{ formatTokens(m.max_context_tokens) }}
							</span>
							<span v-if="m.max_output_tokens" class="flex items-center gap-1">
								<Icon name="arrowUp" size="xs" class="text-gray-300" />
								输出 {{ formatTokens(m.max_output_tokens) }}
							</span>
							<span v-if="m.max_concurrency" class="flex items-center gap-1">
								<Icon name="chart" size="xs" class="text-gray-300" />
								并发 {{ m.max_concurrency }}
							</span>
						</div>

						<div v-if="m.discount_ratio && m.discount_ratio < 1" class="mt-3 pt-3 border-t border-gray-100">
							<span class="badge badge-success">
								{{ (m.discount_ratio * 100).toFixed(0) }}% 折扣
							</span>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
