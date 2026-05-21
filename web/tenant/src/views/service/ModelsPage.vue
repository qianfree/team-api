<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

interface PricingTierItem {
	min_tokens: number
	max_tokens: number | null
	input_price: number
	output_price: number
}

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
	input_price: number | null
	output_price: number | null
	cache_read_price: number | null
	cache_creation_price: number | null
	pricing_tiers: PricingTierItem[]
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
const expandedPricingId = ref<number | null>(null)

function togglePricingExpand(id: number) {
	expandedPricingId.value = expandedPricingId.value === id ? null : id
}

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

const categoryBadgeStyle: Record<string, string> = {
	chat: 'background:rgba(20,184,166,0.1);color:#0d9488',
	embedding: 'background:rgba(139,92,246,0.1);color:#7c3aed',
	image: 'background:rgba(245,158,11,0.1);color:#d97706',
	audio: 'background:rgba(16,185,129,0.1);color:#059669',
	rerank: 'background:rgba(107,114,128,0.1);color:#4b5563',
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

function formatPrice(n: number | null): string {
	if (n == null || n === 0) return '-'
	return '$' + n.toFixed(2)
}

function formatTokenRange(min: number, max: number | null): string {
	const fmtMin = formatTokens(min)
	if (max == null) return fmtMin + '+'
	return fmtMin + ' ~ ' + formatTokens(max)
}

function hasPricing(m: ModelItem): boolean {
	if (m.billing_mode === 'per_request' && m.per_request_price != null) return true
	if (m.billing_mode === 'token' && (m.input_price != null || m.output_price != null)) return true
	if (m.billing_mode === 'tiered' && m.pricing_tiers?.length > 0) return true
	return false
}

function getTieredStartPrice(m: ModelItem): string {
	if (!m.pricing_tiers?.length) return '-'
	const first = m.pricing_tiers[0]
	if (first.input_price > 0 && first.output_price > 0) {
		return formatPrice(Math.min(first.input_price, first.output_price))
	}
	if (first.input_price > 0) return formatPrice(first.input_price)
	return formatPrice(first.output_price)
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

						<!-- Header: Category badge + Model name -->
						<div class="flex items-center gap-2" :class="{ 'pr-14': m.discount_ratio && m.discount_ratio < 1 }">
							<span
								class="shrink-0 px-1.5 py-0.5 text-[10px] font-medium rounded"
								:style="categoryBadgeStyle[m.category] || categoryBadgeStyle.rerank"
							>
								{{ categoryLabel[m.category] || m.category }}
							</span>
							<h3 class="text-sm font-semibold text-gray-900 truncate">{{ m.model_name || m.model_id }}</h3>
						</div>

						<!-- Model ID + Copy -->
						<div class="flex items-center gap-1 mt-1">
							<span class="text-xs text-gray-400 font-mono truncate">{{ m.model_id }}</span>
							<button
								class="shrink-0 p-0.5 rounded text-gray-300 hover:text-primary-500 transition-colors duration-150"
								title="复制模型编码"
								@click="copyModelId(m.model_id)"
							>
								<Icon v-if="copiedId !== m.model_id" name="copy" size="xs" />
								<Icon v-else name="check" size="xs" class="text-emerald-500" />
							</button>
						</div>

						<!-- Description (only if exists) -->
						<p v-if="m.description" class="mt-2.5 text-xs text-gray-500 line-clamp-2 leading-relaxed">
							{{ m.description }}
						</p>

						<!-- Key specs (dot-separated) -->
						<div class="mt-2.5 flex items-center gap-1.5 text-[11px] text-gray-400">
							<span v-if="m.max_context_tokens">{{ formatTokens(m.max_context_tokens) }} 上下文</span>
							<span v-if="m.max_context_tokens && m.max_output_tokens" class="text-gray-200">·</span>
							<span v-if="m.max_output_tokens">{{ formatTokens(m.max_output_tokens) }} 输出</span>
							<template v-if="m.max_concurrency">
								<span class="text-gray-200">·</span>
								<span>并发 {{ m.max_concurrency }}</span>
							</template>
						</div>

						<!-- Capabilities (max 4 visible) -->
						<div v-if="parseCapabilities(m.capabilities).length" class="mt-2.5 flex flex-wrap gap-1">
							<span
								v-for="cap in parseCapabilities(m.capabilities).slice(0, 4)"
								:key="cap"
								class="inline-flex items-center rounded-md bg-primary-50 px-1.5 py-0.5 text-[10px] font-medium text-primary-600"
							>
								{{ capabilityLabel[cap] || cap }}
							</span>
							<span
								v-if="parseCapabilities(m.capabilities).length > 4"
								class="inline-flex items-center rounded-md bg-gray-50 px-1.5 py-0.5 text-[10px] text-gray-400"
							>
								+{{ parseCapabilities(m.capabilities).length - 4 }}
							</span>
						</div>

						<!-- Pricing (pushed to bottom) -->
						<div v-if="hasPricing(m)" class="mt-auto pt-3 border-t border-gray-100">
							<template v-if="m.billing_mode === 'token'">
								<div class="flex items-baseline gap-3 text-xs">
									<span>
										<span class="text-gray-400">输入</span>
										<span class="ml-1 font-semibold text-gray-800">{{ formatPrice(m.input_price) }}</span>
									</span>
									<span>
										<span class="text-gray-400">输出</span>
										<span class="ml-1 font-semibold text-gray-800">{{ formatPrice(m.output_price) }}</span>
									</span>
									<span class="text-gray-300">/1M tokens</span>
								</div>
							</template>

							<template v-else-if="m.billing_mode === 'per_request'">
								<div class="text-xs">
									<span class="font-semibold text-gray-800">{{ formatPrice(m.per_request_price) }}</span>
									<span class="text-gray-400"> /次</span>
								</div>
							</template>

							<template v-else-if="m.billing_mode === 'tiered'">
								<div class="flex items-center justify-between">
									<div class="text-xs">
										<span class="font-semibold text-gray-800">{{ getTieredStartPrice(m) }}</span>
										<span class="text-gray-400"> 起 /1M tokens</span>
									</div>
									<button
										class="flex items-center gap-0.5 text-xs text-primary-500 hover:text-primary-600 transition-colors"
										@click.stop="togglePricingExpand(m.id)"
									>
										{{ expandedPricingId === m.id ? '收起' : '阶梯详情' }}
										<Icon :name="expandedPricingId === m.id ? 'chevronUp' : 'chevronDown'" size="xs" />
									</button>
								</div>
								<div v-if="expandedPricingId === m.id && m.pricing_tiers?.length" class="mt-2 bg-gray-50 rounded-lg p-2.5 space-y-1.5">
									<div
										v-for="(tier, idx) in m.pricing_tiers"
										:key="idx"
										class="flex items-center justify-between text-xs"
									>
										<span class="text-gray-500 font-mono">{{ formatTokenRange(tier.min_tokens, tier.max_tokens) }}</span>
										<span>
											<span class="text-gray-400">输入</span>
											<span class="font-medium text-gray-700 ml-0.5">{{ formatPrice(tier.input_price) }}</span>
											<span class="text-gray-300 mx-1">|</span>
											<span class="text-gray-400">输出</span>
											<span class="font-medium text-gray-700 ml-0.5">{{ formatPrice(tier.output_price) }}</span>
										</span>
									</div>
								</div>
							</template>
						</div>
					</div>
				</div>
			</div>
		</div>
	</div>
</template>
