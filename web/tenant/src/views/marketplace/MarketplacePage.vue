<template>
	<div class="marketplace-page">
		<header class="public-header">
			<div class="header-inner">
				<router-link :to="{ name: 'TenantHome' }" class="brand-link" aria-label="返回首页">
					<span class="brand-mark">
						<img src="/favicon.png" :alt="siteName" />
					</span>
					<span class="brand-copy">
						<strong>{{ siteName }}</strong>
						<small>MODEL CATALOG</small>
					</span>
				</router-link>

				<div class="header-actions">
					<router-link v-if="!isLoggedIn" :to="{ name: 'TenantLogin' }" class="text-link">登录</router-link>
					<router-link :to="primaryActionRoute" class="header-cta">
						{{ primaryActionLabel }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
				</div>
			</div>
		</header>

		<main>
			<section id="catalog" ref="catalogSection" class="catalog-section">
				<div class="catalog-heading">
					<div>
						<span class="section-kicker">MODEL MARKETPLACE</span>
						<h2>模型目录</h2>
						<p>按任务类型筛选，快速比较上下文、输出能力与 Token 单价。</p>
					</div>
				</div>

				<div class="catalog-layout">
					<aside class="category-panel">
						<div class="category-panel-title">模型类型</div>
						<div class="category-list" role="list">
							<button
								v-for="category in categories"
								:key="category.key"
								type="button"
								class="category-button"
								:class="{ active: selectedCategory === category.value }"
								:style="getCategoryStyle(category.value || '')"
								@click="selectCategory(category.value)"
							>
								<span class="category-icon"><Icon :name="category.icon" size="sm" /></span>
								<span>
									<strong>{{ category.label }}</strong>
									<small>{{ category.description }}</small>
								</span>
								<Icon name="chevronRight" size="xs" class="category-chevron" />
							</button>
						</div>

						<div class="category-help">
							<span><Icon name="infoCircle" size="sm" /></span>
							<div>
								<strong>不知道怎么选？</strong>
								<p>先从任务类型开始，详情中可查看完整能力与价格。</p>
							</div>
						</div>
					</aside>

					<div class="catalog-main">
						<div class="catalog-toolbar">
							<div class="search-box">
								<Icon name="search" size="md" />
								<input
									ref="searchInputElement"
									v-model="keyword"
									type="search"
									placeholder="搜索模型名称、模型 ID 或能力..."
									aria-label="搜索模型"
									@input="scheduleSearch"
									@keydown.enter.prevent="runSearchNow"
								/>
								<button v-if="keyword" type="button" class="clear-search" aria-label="清除搜索" @click="clearSearch">
									<Icon name="x" size="sm" />
								</button>
							</div>

							<div class="toolbar-meta">
								<span class="active-filter-label">{{ activeCategoryLabel }}</span>
								<div class="view-switch" aria-label="切换展示方式">
									<button type="button" :class="{ active: viewMode === 'grid' }" aria-label="网格视图" @click="viewMode = 'grid'">
										<Icon name="grid" size="sm" />
									</button>
									<button type="button" :class="{ active: viewMode === 'list' }" aria-label="列表视图" @click="viewMode = 'list'">
										<Icon name="document" size="sm" />
									</button>
								</div>
							</div>
						</div>

						<div v-if="loading" class="models-grid" :class="{ 'list-view': viewMode === 'list' }" aria-label="正在加载模型">
							<div v-for="item in pageSize" :key="item" class="model-card skeleton-card">
								<div class="skeleton skeleton-heading"></div>
								<div class="skeleton skeleton-id"></div>
								<div class="skeleton skeleton-line"></div>
								<div class="skeleton skeleton-line short"></div>
								<div class="skeleton skeleton-footer"></div>
							</div>
						</div>

						<div v-else-if="loadError" class="catalog-state">
							<span class="state-icon error"><Icon name="exclamationTriangle" size="lg" /></span>
							<h3>模型列表加载失败</h3>
							<p>网络可能暂时不可用，请稍后重试。</p>
							<button type="button" class="secondary-button dark" @click="loadModels">
								<Icon name="refresh" size="sm" />
								重新加载
							</button>
						</div>

						<div v-else-if="modelList.length === 0" class="catalog-state">
							<span class="state-icon"><Icon name="search" size="lg" /></span>
							<h3>没有找到匹配的模型</h3>
							<p>换一个关键词，或者清除当前分类后再试。</p>
							<button type="button" class="secondary-button dark" @click="resetFilters">
								<Icon name="refresh" size="sm" />
								重置筛选
							</button>
						</div>

						<div v-else class="models-grid" :class="{ 'list-view': viewMode === 'list' }">
							<article
								v-for="model in modelList"
								:key="model.model_id"
								class="model-card"
								:style="getCategoryStyle(model.category)"
							>
								<div class="model-card-accent"></div>
								<div class="model-card-header">
									<span class="model-mark">
										<Icon :name="getCategoryMeta(model.category).icon" size="md" />
									</span>
									<div class="model-title-group">
										<div class="model-provider">{{ getProvider(model) }}</div>
										<div class="model-name-line">
											<h3>{{ model.model_name || model.model_id }}</h3>
											<span v-if="model.discount_label" class="promo-badge">{{ model.discount_label }}</span>
											<button type="button" aria-label="复制模型 ID" title="复制模型 ID" @click="copyModelId(model.model_id)">
												<Icon name="copy" size="xs" />
											</button>
										</div>
									</div>
								</div>

								<div class="model-id-row">
									<code>{{ model.model_id }}</code>
								</div>

								<p class="model-description">{{ model.description || '暂无模型描述，点击查看技术规格与计费信息。' }}</p>

								<div class="model-specs">
									<div>
										<span>上下文</span>
										<strong>{{ formatTokens(model.max_context_tokens) }}</strong>
									</div>
									<div>
										<span>最大输出</span>
										<strong>{{ formatTokens(model.max_output_tokens) }}</strong>
									</div>
								</div>

								<div v-if="model.time_prices?.length" class="time-price-list">
									<div v-for="(tp, idx) in model.time_prices" :key="idx" class="time-price-row">
										<span>{{ tp.name }}<small>{{ timeWindow(tp) }}</small></span>
										<strong>{{ timePriceText(model, tp) }}</strong>
									</div>
								</div>
								<p v-if="model.price_change_note" class="price-change-note">
									<Icon name="infoCircle" size="xs" />
									{{ model.price_change_note }}
								</p>
								
								<div class="model-card-footer">
									<template v-if="model.billing_mode === 'per_request'">
										<div class="price-pair">
											<div>
												<span>调用单价</span>
												<strong>{{ formatPrice(model.per_request_price) }}</strong>
											</div>
										<small>/ 次</small>
										</div>
									</template>
									<div v-else class="price-pair">
										<div>
											<span>输入</span>
											<strong>{{ formatPrice(model.input_price) }}</strong>
										</div>
										<i></i>
										<div>
											<span>输出</span>
											<strong>{{ formatPrice(model.output_price) }}</strong>
										</div>
										<small>{{ model.billing_mode === 'tiered' ? '/ 1M tokens 起' : '/ 1M tokens' }}</small>
									</div>
									<button type="button" class="detail-button" @click="openDetail(model)">
										查看详情
										<Icon name="arrowRight" size="sm" />
									</button>
								</div>
							</article>
						</div>

						<div v-if="totalPages > 1 && !loading && !loadError" class="pagination-row">
							<span>第 {{ currentPage }} / {{ totalPages }} 页</span>
							<n-pagination
								v-model:page="currentPage"
								:page-count="totalPages"
								:page-size="pageSize"
								show-size-picker
								:page-sizes="[9, 18, 36]"
								@update:page="handlePageChange"
								@update:page-size="handlePageSizeChange"
							/>
						</div>
					</div>
				</div>
			</section>

			<section class="bottom-cta">
				<div class="bottom-cta-copy">
					<span class="cta-icon"><Icon name="bolt" size="lg" /></span>
					<div>
						<span>ONE API, EVERY MODEL</span>
						<h2>选好模型，下一步就是开始调用。</h2>
						<p>创建 API 密钥，使用熟悉的 OpenAI SDK 即可接入。</p>
					</div>
				</div>
				<div class="bottom-cta-actions">
					<router-link :to="primaryActionRoute" class="light-button">
						{{ primaryActionLabel }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
					<router-link :to="{ name: 'TenantDocs' }" class="ghost-button">阅读 API 文档</router-link>
				</div>
			</section>
		</main>

		<footer class="public-footer">
			<div class="footer-inner">
				<div class="footer-brand">
					<img src="/favicon.png" :alt="siteName" />
					<span>{{ siteName }}</span>
				</div>
				<p>统一的大模型 API 接入与管理平台</p>
				<span>© {{ currentYear }} qianfree · AGPL-3.0</span>
			</div>
		</footer>

		<n-modal v-model:show="detailModalVisible" :mask-closable="!detailLoading" transform-origin="center">
			<section v-if="selectedModel" class="detail-modal" :style="getCategoryStyle(selectedModel.category)">
				<button type="button" class="modal-close" aria-label="关闭详情" @click="closeDetail">
					<Icon name="x" size="sm" />
				</button>

				<div class="detail-hero">
					<div class="detail-identity">
						<span class="detail-model-mark"><Icon :name="getCategoryMeta(selectedModel.category).icon" size="lg" /></span>
						<div>
							<span class="detail-provider">{{ getProvider(selectedModel) }}</span>
							<h2>{{ selectedModel.model_name || selectedModel.model_id }}</h2>
							<button type="button" class="detail-model-id" @click="copyModelId(selectedModel.model_id)">
								<code>{{ selectedModel.model_id }}</code>
								<Icon name="copy" size="xs" />
							</button>
						</div>
					</div>
					<span v-if="selectedModel.discount_label" class="promo-badge large">{{ selectedModel.discount_label }}</span>
					<span class="detail-category">{{ getCategoryMeta(selectedModel.category).label }}</span>
				</div>

				<div v-if="detailLoading" class="detail-loading">
					<span class="spinner"></span>
					<p>正在加载完整模型信息...</p>
				</div>

				<div v-else class="detail-body">
					<div class="detail-main-column">
						<section class="detail-section">
							<span class="detail-section-label">模型简介</span>
							<p class="detail-description">{{ selectedModel.description || '暂无模型描述。' }}</p>
						</section>
						<section class="detail-section">
							<span class="detail-section-label">计费信息<em v-if="selectedModel.billing_mode === 'tiered'" class="price-tiered-hint">阶梯计费 · 以下为首档起价</em></span>
							<div class="price-tile-grid">
								<template v-if="selectedModel.billing_mode === 'per_request'">
									<div class="price-tile accent">
										<span>单次调用</span>
										<strong>{{ formatPrice(selectedModel.per_request_price) }}</strong>
										<small>USD / 次</small>
									</div>
								</template>
								<template v-else>
									<div class="price-tile accent">
										<span>输入</span>
										<strong>{{ formatPrice(selectedModel.input_price) }}</strong>
										<small>USD / 1M tokens</small>
									</div>
									<div class="price-tile">
										<span>输出</span>
										<strong>{{ formatPrice(selectedModel.output_price) }}</strong>
										<small>USD / 1M tokens</small>
									</div>
									<div v-if="selectedModel.cache_read_price" class="price-tile">
										<span>缓存读取</span>
										<strong>{{ formatPrice(selectedModel.cache_read_price) }}</strong>
										<small>USD / 1M tokens</small>
									</div>
									<div v-if="selectedModel.cache_creation_price" class="price-tile">
										<span>缓存创建</span>
										<strong>{{ formatPrice(selectedModel.cache_creation_price) }}</strong>
										<small>USD / 1M tokens</small>
									</div>
								</template>
							</div>
							<div v-if="selectedModel.time_prices?.length" class="detail-time-prices">
								<div class="detail-time-head">分时段价格<small>命中时段按乘数计费，未命中按默认价</small></div>
								<div v-for="(tp, idx) in selectedModel.time_prices" :key="idx" class="detail-time-row">
									<span>{{ tp.name }}<small>{{ timeWindow(tp) }}</small></span>
									<strong>{{ timePriceText(selectedModel, tp) }}</strong>
								</div>
							</div>
							<p v-if="selectedModel.price_change_note" class="price-change-note">
								<Icon name="infoCircle" size="xs" />
								{{ selectedModel.price_change_note }}
							</p>
						</section>
						<section class="detail-section">
							<span class="detail-section-label">能力支持</span>
							<div class="detail-capabilities">
								<span v-for="capability in getCapabilityList(selectedModel, 12)" :key="capability">
									<Icon name="checkCircle" size="sm" />
									{{ capability }}
								</span>
								<span v-if="!getCapabilityList(selectedModel, 12).length">
									<Icon name="checkCircle" size="sm" />
									标准 API 接入
								</span>
							</div>
						</section>


					</div>

					<aside class="detail-side-column">
						<div class="detail-spec-card">
							<div class="detail-spec-title">技术规格</div>
							<div class="detail-spec-row">
								<span><Icon name="document" size="sm" /> 最大上下文</span>
								<strong>{{ formatTokens(selectedModel.max_context_tokens) }} tokens</strong>
							</div>
							<div class="detail-spec-row">
								<span><Icon name="arrowRight" size="sm" /> 最大输出</span>
								<strong>{{ formatTokens(selectedModel.max_output_tokens) }} tokens</strong>
							</div>
							<div class="detail-spec-row">
								<span><Icon name="grid" size="sm" /> 模型类别</span>
								<strong>{{ getCategoryMeta(selectedModel.category).label }}</strong>
							</div>
							<div class="detail-spec-row">
								<span><Icon name="creditCard" size="sm" /> 计费模式</span>
								<strong>{{ billingModeLabel(selectedModel.billing_mode) }}</strong>
							</div>
						</div>
						<div v-if="selectedModel.tags?.length" class="detail-spec-card">
							<div class="detail-spec-title">模型标签</div>
							<div class="detail-tags">
								<span v-for="tag in selectedModel.tags" :key="tag">{{ tag }}</span>
							</div>
						</div>

					</aside>
				</div>

				<div class="detail-footer">
					<p><Icon name="infoCircle" size="sm" /> 实际可用范围以账户套餐与 API 密钥权限为准</p>
					<router-link :to="primaryActionRoute" class="primary-button" @click="detailModalVisible = false">
						{{ isLoggedIn ? '前往使用' : '创建账户' }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
				</div>
			</section>
		</n-modal>
	</div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NModal, NPagination } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import { getMarketplaceModelDetail, getMarketplaceModels, type MarketplaceModel, type TimePriceItem } from '@/api/marketplace'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useSeo } from '@/composables/useSeo'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { toast } from '@/utils/toast'

type ViewMode = 'grid' | 'list'

interface CategoryMeta {
	key: string
	value: string | null
	label: string
	description: string
	icon: string
	accent: string
	soft: string
}

const authStore = useTenantAuthStore()
const { settings: publicSettings, fetchSettings } = usePublicSettings()

const siteName = computed(() => publicSettings.value.site_name || 'Team-API')
const isLoggedIn = computed(() => authStore.isLoggedIn)
const primaryActionRoute = computed(() => {
	if (isLoggedIn.value) return { name: 'TenantModels' }
	return publicSettings.value.register_enabled === false ? { name: 'TenantLogin' } : { name: 'TenantRegister' }
})
const primaryActionLabel = computed(() => {
	if (isLoggedIn.value) return '进入控制台'
	return publicSettings.value.register_enabled === false ? '登录使用' : '免费开始'
})

const currentYear = new Date().getFullYear()

const categories: CategoryMeta[] = [
	{ key: 'all', value: null, label: '全部模型', description: '浏览完整模型目录', icon: 'grid', accent: '#0f766e', soft: '#ecfdf5' },
	{ key: 'chat', value: 'chat', label: '对话与推理', description: '文本生成、推理与工具调用', icon: 'chat', accent: '#0f766e', soft: '#ecfdf5' },
	{ key: 'embedding', value: 'embedding', label: '向量嵌入', description: '语义检索与知识库构建', icon: 'cube', accent: '#6d5bd0', soft: '#f3f0ff' },
	{ key: 'image', value: 'image', label: '图像生成', description: '文生图、图像理解与编辑', icon: 'eye', accent: '#c2416c', soft: '#fff0f5' },
	{ key: 'audio', value: 'audio', label: '音频处理', description: '语音识别、合成与音乐生成', icon: 'bell', accent: '#b45309', soft: '#fff7e8' },
	{ key: 'video', value: 'video', label: '视频生成', description: '文本或图像驱动的视频创作', icon: 'play', accent: '#2563a8', soft: '#eff6ff' },
]

const defaultCategory: CategoryMeta = {
	key: 'other', value: null, label: '其他', description: '其他模型能力', icon: 'bolt', accent: '#475569', soft: '#f1f5f9',
}

const keyword = ref('')
const selectedCategory = ref<string | null>(null)
const viewMode = ref<ViewMode>('grid')
const loading = ref(false)
const loadError = ref(false)
const modelList = ref<MarketplaceModel[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(9)
const catalogSection = ref<HTMLElement | null>(null)
const searchInputElement = ref<HTMLInputElement | null>(null)
const detailModalVisible = ref(false)
const detailLoading = ref(false)
const selectedModel = ref<MarketplaceModel | null>(null)

let searchTimer: ReturnType<typeof setTimeout> | null = null
let listRequestId = 0
let detailRequestId = 0

const totalPages = computed(() => Math.ceil(total.value / pageSize.value))
const activeCategoryLabel = computed(() => categories.find(category => category.value === selectedCategory.value)?.label || '全部模型')

function getCategoryMeta(category: string): CategoryMeta {
	return categories.find(item => item.value === category) || defaultCategory
}

function getCategoryStyle(category: string): Record<string, string> {
	const meta = getCategoryMeta(category)
	return {
		'--model-accent': meta.accent,
		'--model-soft': meta.soft,
	}
}

function getProvider(model: MarketplaceModel): string {
	const value = `${model.model_id} ${model.model_name || ''}`.toLowerCase()
	const providers: Array<[string[], string]> = [
		[['gpt', 'openai', 'o1-', 'o3-', 'o4-'], 'OpenAI'],
		[['claude', 'anthropic'], 'Anthropic'],
		[['gemini', 'google'], 'Google'],
		[['deepseek'], 'DeepSeek'],
		[['qwen', '通义'], 'Alibaba Cloud'],
		[['glm', '智谱'], 'Zhipu AI'],
		[['moonshot', 'kimi'], 'Moonshot AI'],
		[['minimax'], 'MiniMax'],
		[['doubao', '豆包'], 'ByteDance'],
		[['llama'], 'Meta'],
		[['mistral', 'mixtral'], 'Mistral AI'],
		[['cohere', 'command-r'], 'Cohere'],
	]
	return providers.find(([keywords]) => keywords.some(item => value.includes(item)))?.[1] || `${getCategoryMeta(model.category).label}模型`
}

function getCapabilityList(model: MarketplaceModel, limit = 4): string[] {
	if (!model.capabilities || typeof model.capabilities !== 'object') return []
	const labels: Record<string, string> = {
		streaming: '流式输出',
		function_calling: '工具调用',
		tool_calling: '工具调用',
		vision: '视觉理解',
		multimodal: '多模态',
		json_mode: 'JSON 模式',
		reasoning: '深度推理',
		web_search: '联网搜索',
		image_generation: '图像生成',
		audio: '音频能力',
	}
	return Object.entries(model.capabilities)
		.filter(([, value]) => value === true || value === 1 || value === 'true')
		.map(([key]) => labels[key] || key.replaceAll('_', ' '))
		.filter((item, index, list) => list.indexOf(item) === index)
		.slice(0, limit)
}

function formatTokens(tokens?: number): string {
	const value = Number(tokens || 0)
	if (!value) return '—'
	if (value >= 1_000_000) return `${Number((value / 1_000_000).toFixed(1))}M`
	if (value >= 1_000) return `${Number((value / 1_000).toFixed(1))}K`
	return value.toLocaleString('en-US')
}

function formatPrice(price?: number): string {
	const value = Number(price)
	if (!Number.isFinite(value)) return '—'
	// 最多保留 6 位小数（与后端 NUMERIC(20,10) 精度对齐），自动去掉末尾的 0
	return `$${Number(value.toFixed(6))}`
}

function billingModeLabel(mode?: string): string {
	if (mode === 'per_request') return '按次计费'
	if (mode === 'tiered') return '阶梯计费'
	return '按量计费'
}

// ── 时段价目展示辅助（价格为后端换算好的展示价，前端直接渲染） ──

function daysLabel(days?: number[] | null): string {
	if (!days || days.length === 0) return '每天'
	const sorted = [...days].sort((a, b) => a - b)
	if (sorted.join(',') === '1,2,3,4,5') return '工作日'
	if (sorted.join(',') === '6,7') return '周末'
	const names = ['', '周一', '周二', '周三', '周四', '周五', '周六', '周日']
	return sorted.map((d) => names[d] || String(d)).join('、')
}

function timeWindow(tp: TimePriceItem): string {
	const parts: string[] = [daysLabel(tp.days)]
	if (tp.start_time && tp.end_time) parts.push(`${tp.start_time}~${tp.end_time}`)
	else parts.push('全天')
	if (tp.valid_from) parts.push(tp.valid_to ? `${tp.valid_from}~${tp.valid_to}` : `${tp.valid_from} 起`)
	return parts.join(' · ')
}

function timePriceText(model: MarketplaceModel, tp: TimePriceItem): string {
	if (model.billing_mode === 'per_request') return `${formatPrice(tp.per_request_price ?? undefined)} /次`
	return `输入 ${formatPrice(tp.input_price ?? undefined)} · 输出 ${formatPrice(tp.output_price ?? undefined)}${model.billing_mode === 'tiered' ? ' 起' : ''}`
}

async function loadModels(): Promise<void> {
	const requestId = ++listRequestId
	loading.value = true
	loadError.value = false

	try {
		const response = await getMarketplaceModels({
			keyword: keyword.value.trim() || undefined,
			category: selectedCategory.value || undefined,
			page: currentPage.value,
			page_size: pageSize.value,
		})
		if (requestId !== listRequestId) return
		modelList.value = response.list || []
		total.value = response.total || 0
	} catch {
		if (requestId !== listRequestId) return
		modelList.value = []
		total.value = 0
		loadError.value = true
	} finally {
		if (requestId === listRequestId) loading.value = false
	}
}

function scheduleSearch(): void {
	if (searchTimer) clearTimeout(searchTimer)
	searchTimer = setTimeout(() => {
		currentPage.value = 1
		loadModels()
	}, 320)
}

function runSearchNow(): void {
	if (searchTimer) clearTimeout(searchTimer)
	currentPage.value = 1
	loadModels()
}

function clearSearch(): void {
	keyword.value = ''
	runSearchNow()
	searchInputElement.value?.focus()
}

function selectCategory(category: string | null): void {
	if (selectedCategory.value === category) return
	selectedCategory.value = category
	currentPage.value = 1
	loadModels()
}

function resetFilters(): void {
	keyword.value = ''
	selectedCategory.value = null
	currentPage.value = 1
	loadModels()
}

function handlePageChange(page: number): void {
	currentPage.value = page
	loadModels()
	catalogSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function handlePageSizeChange(size: number): void {
	pageSize.value = size
	currentPage.value = 1
	loadModels()
}

async function openDetail(model: MarketplaceModel): Promise<void> {
	selectedModel.value = model
	detailModalVisible.value = true
	detailLoading.value = true
	const requestId = ++detailRequestId

	try {
		const detail = await getMarketplaceModelDetail(model.model_id)
		if (requestId === detailRequestId && detailModalVisible.value) selectedModel.value = detail
	} catch {
		// 列表数据足够展示基础详情，接口失败时保持已有内容。
	} finally {
		if (requestId === detailRequestId) detailLoading.value = false
	}
}

function closeDetail(): void {
	detailRequestId += 1
	detailModalVisible.value = false
	detailLoading.value = false
}

async function copyModelId(modelId: string): Promise<void> {
	try {
		if (navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(modelId)
		} else {
			const textarea = document.createElement('textarea')
			textarea.value = modelId
			textarea.style.position = 'fixed'
			textarea.style.opacity = '0'
			document.body.appendChild(textarea)
			textarea.select()
			document.execCommand('copy')
			textarea.remove()
		}
		toast.success('模型 ID 已复制')
	} catch {
		toast.warning('复制失败，请手动复制')
	}
}

onMounted(() => {
	authStore.loadFromStorage()
	fetchSettings()
	loadModels()
})

onBeforeUnmount(() => {
	if (searchTimer) clearTimeout(searchTimer)
	listRequestId += 1
	detailRequestId += 1
})

useSeo({
	title: '模型广场 - Team-API',
	description: '浏览主流 AI 模型，比较模型能力、上下文窗口与 Token 价格，通过统一 OpenAI 兼容接口快速接入。',
	keywords: '大模型,AI 模型,模型价格,OpenAI,Claude,Gemini,DeepSeek,模型 API',
})
</script>

<style scoped>
.marketplace-page {
	--ink: #10231f;
	--muted: #64706c;
	--line: #dce5e1;
	--paper: #f7faf8;
	--surface: #ffffff;
	--green: #0f766e;
	--green-dark: #0b4f4a;
	--lime: #c8f46a;
	min-height: 100vh;
	background: var(--paper);
	color: var(--ink);
}

.public-header {
	position: sticky;
	top: 0;
	z-index: 40;
	border-bottom: 1px solid rgba(220, 229, 225, 0.82);
	background: rgba(247, 250, 248, 0.88);
	backdrop-filter: blur(18px) saturate(1.2);
}

.header-inner,
.catalog-section,
.bottom-cta,
.footer-inner {
	width: min(1440px, calc(100% - 48px));
	margin: 0 auto;
}

.header-inner {
	display: flex;
	height: 76px;
	align-items: center;
	justify-content: space-between;
	gap: 28px;
}

.brand-link,
.brand-copy,
.header-actions,
.header-nav,
.model-card-header,
.model-id-row,
.model-card-footer,
.toolbar-meta,
.pagination-row,
.bottom-cta-copy,
.bottom-cta-actions,
.footer-inner,
.footer-brand,
.detail-identity,
.detail-footer {
	display: flex;
	align-items: center;
}

.brand-link {
	gap: 11px;
	color: var(--ink);
	text-decoration: none;
}

.brand-mark {
	display: grid;
	width: 42px;
	height: 42px;
	place-items: center;
	border: 1px solid var(--line);
	border-radius: 13px;
	background: #fff;
	box-shadow: 0 8px 24px rgba(15, 55, 47, 0.08);
}

.brand-mark img {
	width: 27px;
	height: 27px;
	border-radius: 7px;
	object-fit: contain;
}

.brand-copy {
	align-items: flex-start;
	flex-direction: column;
	line-height: 1;
}

.brand-copy strong {
	font-size: 16px;
	font-weight: 800;
	letter-spacing: -0.02em;
}

.brand-copy small {
	margin-top: 5px;
	color: #8b9692;
	font-size: 9px;
	font-weight: 800;
	letter-spacing: 0.18em;
}

.header-nav {
	gap: 32px;
}

.header-nav a,
.text-link {
	position: relative;
	color: #596762;
	font-size: 14px;
	font-weight: 600;
	text-decoration: none;
	transition: color 160ms ease;
}

.header-nav a:hover,
.header-nav a.is-active,
.text-link:hover {
	color: var(--ink);
}

.header-nav a.is-active::after {
	position: absolute;
	right: 0;
	bottom: -12px;
	left: 0;
	height: 2px;
	border-radius: 2px;
	background: var(--green);
	content: '';
}

.header-actions {
	gap: 18px;
}

.header-cta,
.primary-button,
.secondary-button,
.light-button,
.ghost-button,
.detail-button {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: 9px;
	border: 0;
	text-decoration: none;
	cursor: pointer;
	transition: transform 180ms ease, box-shadow 180ms ease, background-color 180ms ease, border-color 180ms ease;
}

.header-cta {
	min-height: 42px;
	padding: 0 17px;
	border-radius: 12px;
	background: var(--ink);
	color: #fff;
	font-size: 13px;
	font-weight: 700;
}

.header-cta:hover,
.primary-button:hover,
.light-button:hover {
	transform: translateY(-2px);
	box-shadow: 0 14px 30px rgba(15, 55, 47, 0.18);
}

.primary-button,
.secondary-button {
	min-height: 50px;
	padding: 0 21px;
	border-radius: 14px;
	font-size: 14px;
	font-weight: 750;
}

.primary-button {
	background: var(--green-dark);
	color: #fff;
}

.secondary-button {
	border: 1px solid #d5dfdb;
	background: rgba(255, 255, 255, 0.76);
	color: var(--ink);
}

.secondary-button:hover {
	transform: translateY(-2px);
	border-color: #aebfba;
	background: #fff;
}

.secondary-button.dark {
	min-height: 44px;
	padding: 0 17px;
	background: #fff;
}

.catalog-section {
	padding: 80px 0 100px;
	scroll-margin-top: 88px;
}

.catalog-heading {
	display: flex;
	align-items: flex-end;
	justify-content: space-between;
	gap: 24px;
	margin-bottom: 34px;
}

.section-kicker {
	color: var(--green);
	font-size: 10px;
	font-weight: 850;
	letter-spacing: 0.18em;
}

.catalog-heading h2 {
	margin: 8px 0 5px;
	font-size: clamp(34px, 4vw, 48px);
	font-weight: 820;
	letter-spacing: -0.045em;
}

.catalog-heading p {
	margin: 0;
	color: var(--muted);
	font-size: 14px;
}

.catalog-layout {
	display: grid;
	grid-template-columns: 230px minmax(0, 1fr);
	align-items: start;
	gap: 30px;
}

.category-panel {
	position: sticky;
	top: 100px;
}

.category-panel-title {
	margin-bottom: 10px;
	padding: 0 8px;
	color: #77837f;
	font-size: 11px;
	font-weight: 800;
	letter-spacing: 0.08em;
}

.category-list {
	display: flex;
	flex-direction: column;
	gap: 4px;
}

.category-button {
	display: grid;
	width: 100%;
	grid-template-columns: 36px minmax(0, 1fr) 14px;
	align-items: center;
	gap: 10px;
	padding: 10px;
	border: 1px solid transparent;
	border-radius: 13px;
	background: transparent;
	color: #52605b;
	text-align: left;
	cursor: pointer;
	transition: background-color 150ms ease, border-color 150ms ease, color 150ms ease;
}

.category-button:hover {
	background: rgba(255, 255, 255, 0.72);
}

.category-button.active {
	border-color: #d8e3df;
	background: #fff;
	color: var(--ink);
	box-shadow: 0 9px 30px rgba(24, 58, 51, 0.06);
}

.category-icon {
	display: grid;
	width: 34px;
	height: 34px;
	place-items: center;
	border-radius: 10px;
	background: var(--model-soft);
	color: var(--model-accent);
}

.category-button > span:nth-child(2) {
	display: flex;
	min-width: 0;
	flex-direction: column;
	gap: 2px;
}

.category-button strong {
	font-size: 12px;
	font-weight: 700;
}

.category-button small {
	overflow: hidden;
	color: #64748b;
	font-size: 9px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.category-chevron {
	color: #b0bab6;
	opacity: 0;
	transition: opacity 150ms ease;
}

.category-button.active .category-chevron,
.category-button:hover .category-chevron {
	opacity: 1;
}

.category-help {
	display: flex;
	gap: 10px;
	margin-top: 22px;
	padding: 15px;
	border: 1px solid #d8e6e0;
	border-radius: 15px;
	background: #eef7f3;
}

.category-help > span {
	display: grid;
	width: 28px;
	height: 28px;
	flex: 0 0 auto;
	place-items: center;
	border-radius: 9px;
	background: #d7eee5;
	color: var(--green);
}

.category-help strong {
	font-size: 11px;
}

.category-help p {
	margin: 4px 0 0;
	color: #64748b;
	font-size: 9px;
	line-height: 1.55;
}

.catalog-main {
	min-width: 0;
}

.catalog-toolbar {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	margin-bottom: 18px;
}

.search-box {
	position: relative;
	display: flex;
	width: min(520px, 100%);
	align-items: center;
}

.search-box > svg {
	position: absolute;
	left: 15px;
	color: #85918d;
	pointer-events: none;
}

.search-box input {
	width: 100%;
	height: 48px;
	padding: 0 48px 0 46px;
	border: 1px solid var(--line);
	border-radius: 14px;
	outline: none;
	background: #fff;
	color: var(--ink);
	font-size: 13px;
	box-shadow: 0 8px 24px rgba(24, 58, 51, 0.035);
	transition: border-color 160ms ease, box-shadow 160ms ease;
}

/* 隐藏 type="search" 自带的浏览器原生清除按钮，只保留自定义的清除按钮 */
.search-box input::-webkit-search-cancel-button,
.search-box input::-webkit-search-decoration {
	-webkit-appearance: none;
	appearance: none;
}

.search-box input:focus {
	border-color: #63a79e;
	box-shadow: 0 0 0 4px rgba(15, 118, 110, 0.1), 0 10px 28px rgba(24, 58, 51, 0.06);
}

.search-box input::placeholder {
	color: #9aa5a1;
}

.clear-search {
	position: absolute;
	right: 11px;
	display: grid;
	width: 28px;
	height: 28px;
	place-items: center;
	border: 0;
	border-radius: 8px;
	background: #f0f4f2;
	color: #6d7a75;
	cursor: pointer;
}

.toolbar-meta {
	gap: 12px;
}

.active-filter-label {
	color: #64748b;
	font-size: 11px;
	font-weight: 700;
	white-space: nowrap;
}

.view-switch {
	display: flex;
	gap: 3px;
	padding: 3px;
	border: 1px solid var(--line);
	border-radius: 11px;
	background: #eef2f0;
}

.view-switch button {
	display: grid;
	width: 34px;
	height: 32px;
	place-items: center;
	border: 0;
	border-radius: 8px;
	background: transparent;
	color: #84908c;
	cursor: pointer;
}

.view-switch button.active {
	background: #fff;
	color: var(--ink);
	box-shadow: 0 3px 10px rgba(24, 58, 51, 0.08);
}

.models-grid {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: 14px;
}

.model-card {
	--model-accent: #0f766e;
	--model-soft: #ecfdf5;
	position: relative;
	display: flex;
	min-width: 0;
	min-height: 336px;
	flex-direction: column;
	overflow: hidden;
	padding: 19px;
	border: 1px solid var(--line);
	border-radius: 18px;
	background: #fff;
	box-shadow: 0 8px 28px rgba(24, 58, 51, 0.04);
	transition: transform 180ms ease, border-color 180ms ease, box-shadow 180ms ease;
}

.model-card:hover {
	transform: translateY(-3px);
	border-color: color-mix(in srgb, var(--model-accent) 38%, #dce5e1);
	box-shadow: 0 18px 42px rgba(24, 58, 51, 0.09);
}

.model-card-accent {
	position: absolute;
	top: 0;
	right: 21px;
	left: 21px;
	height: 2px;
	border-radius: 0 0 3px 3px;
	background: var(--model-accent);
	opacity: 0.78;
}

.model-card-header {
	align-items: flex-start;
	gap: 11px;
}

.model-mark {
	display: grid;
	width: 42px;
	height: 42px;
	flex: 0 0 auto;
	place-items: center;
	border-radius: 12px;
	background: var(--model-soft);
	color: var(--model-accent);
}

.model-title-group {
	min-width: 0;
	flex: 1;
}

.model-name-line {
	display: flex;
	min-width: 0;
	align-items: center;
	gap: 7px;
}

.model-name-line button {
	display: grid;
	width: 25px;
	height: 25px;
	flex: 0 0 auto;
	place-items: center;
	border: 1px solid #e2e8f0;
	border-radius: 7px;
	background: #f8fafc;
	color: #84918d;
	cursor: pointer;
	transition: border-color 150ms ease, background-color 150ms ease, color 150ms ease;
}

.model-name-line button:hover {
	border-color: #99f6e4;
	background: #f0fdfa;
	color: #0d9488;
}

.model-provider {
	margin-bottom: 3px;
	color: #8b9793;
	font-size: 9px;
	font-weight: 600;
	letter-spacing: 0.06em;
	text-transform: uppercase;
}

.model-title-group h3 {
	min-width: 0;
	flex: 0 1 auto;
	overflow: hidden;
	margin: 0;
	color: var(--ink);
	font-size: 15px;
	font-weight: 700;
	letter-spacing: -0.025em;
	line-height: 1.35;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.model-id-row {
	align-self: flex-start;
	max-width: 100%;
	gap: 5px;
	margin-top: 15px;
	padding: 6px 9px;
	border: 1px solid #e4ebe8;
	border-radius: 8px;
	background: #f5f8f6;
}

.model-id-row code {
	overflow: hidden;
	color: #64716c;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 9px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.model-description {
	display: -webkit-box;
	min-height: 62px;
	overflow: hidden;
	-webkit-box-orient: vertical;
	-webkit-line-clamp: 3;
	margin: 15px 0 0;
	color: #64716c;
	font-size: 11px;
	line-height: 1.78;
}

.model-specs {
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 7px;
	margin-top: auto;
	padding-top: 14px;
}

.model-specs > div {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 8px;
	padding: 9px 10px;
	border-radius: 9px;
	background: #f6f8f7;
}

.model-specs span {
	color: #8b9692;
	font-size: 8px;
	font-weight: 500;
}

.model-specs strong {
	font-size: 10px;
	font-weight: 700;
}

.promo-badge {
	display: inline-flex;
	flex: 0 0 auto;
	align-items: center;
	padding: 2px 8px;
	border-radius: 999px;
	background: linear-gradient(135deg, #f43f5e, #fb923c);
	color: #fff;
	font-size: 9px;
	font-weight: 700;
	letter-spacing: 0.02em;
	box-shadow: 0 3px 10px rgba(244, 63, 94, 0.25);
}

.promo-badge.large {
	margin-left: auto;
	padding: 4px 11px;
	font-size: 11px;
}

.price-change-note {
	display: flex;
	align-items: flex-start;
	gap: 6px;
	margin-top: 10px;
	padding: 7px 9px;
	border: 1px solid rgba(180, 83, 9, 0.16);
	border-radius: 9px;
	background: rgba(180, 83, 9, 0.06);
	color: #b45309;
	font-size: 10px;
	line-height: 1.5;
}

.time-price-list {
	display: flex;
	flex-direction: column;
	gap: 4px;
	margin-top: 11px;
	padding-top: 9px;
	border-top: 1px dashed #e2e8f0;
}

.time-price-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 10px;
	color: #64706c;
	font-size: 9px;
}

.time-price-row span {
	display: inline-flex;
	align-items: baseline;
	gap: 5px;
	min-width: 0;
	overflow: hidden;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.time-price-row small {
	flex: 0 0 auto;
	color: #98a29f;
	font-size: 8px;
}

.time-price-row strong {
	flex: 0 0 auto;
	color: #10231f;
	font-size: 9px;
	font-weight: 700;
}

.detail-time-prices {
	display: flex;
	flex-direction: column;
	gap: 6px;
	margin-top: 12px;
	padding-top: 10px;
	border-top: 1px dashed rgba(15, 118, 110, 0.22);
}

.detail-time-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 12px;
	color: #586660;
	font-size: 11px;
}

.detail-time-row span {
	display: inline-flex;
	align-items: baseline;
	gap: 6px;
	min-width: 0;
}

.detail-time-row small {
	color: #98a29f;
	font-size: 9px;
}

.detail-time-row strong {
	flex: 0 0 auto;
	color: #10231f;
	font-size: 11px;
	font-weight: 700;
}

.price-tile-grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(132px, 1fr));
	gap: 10px;
}

.price-tile {
	display: flex;
	flex-direction: column;
	gap: 3px;
	padding: 13px 15px;
	border: 1px solid #e0e8e5;
	border-radius: 13px;
	background: #fafcfb;
}

.price-tile span {
	color: #64706c;
	font-size: 10px;
	font-weight: 600;
}

.price-tile strong {
	color: var(--ink);
	font-size: 17px;
	font-weight: 800;
	letter-spacing: -0.01em;
}

.price-tile small {
	color: #98a29f;
	font-size: 9px;
}

.price-tile.accent {
	border-color: color-mix(in srgb, var(--model-accent) 26%, #dfe7e4);
	background: var(--model-soft);
}

.price-tile.accent strong {
	color: var(--model-accent);
}

.price-tiered-hint {
	margin-left: 7px;
	color: #98a29f;
	font-size: 9px;
	font-style: normal;
	letter-spacing: 0.02em;
	text-transform: none;
}

.detail-time-head {
	display: flex;
	align-items: baseline;
	gap: 8px;
	margin-bottom: 8px;
	color: #0f766e;
	font-size: 10px;
	font-weight: 700;
}

.detail-time-head small {
	color: #98a29f;
	font-size: 9px;
	font-weight: 400;
}

.model-card-footer {
	justify-content: space-between;
	gap: 9px;
	margin-top: 11px;
	padding-top: 12px;
	border-top: 1px solid #edf1ef;
}

.price-pair {
	display: flex;
	min-width: 0;
	align-items: center;
	gap: 7px;
}

.price-pair > div {
	display: flex;
	flex-direction: column;
	gap: 1px;
}

.price-pair span,
.price-pair small {
	color: #98a29f;
	font-size: 7px;
	font-weight: 500;
}

.price-pair strong {
	font-size: 10px;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
}

.price-pair i {
	width: 1px;
	height: 22px;
	background: #e2e8e5;
}

.price-pair small {
	align-self: flex-end;
	margin-bottom: 2px;
	white-space: nowrap;
}

.detail-button {
	min-height: 35px;
	flex: 0 0 auto;
	padding: 0 11px;
	border-radius: 9px;
	background: var(--ink);
	color: #fff;
	font-size: 9px;
	font-weight: 650;
}

.detail-button:hover {
	background: var(--green-dark);
	transform: translateX(2px);
}

.models-grid.list-view {
	grid-template-columns: 1fr;
}

.models-grid.list-view .model-card:not(.skeleton-card) {
	display: grid;
	min-height: 0;
	grid-template-columns: minmax(210px, 1fr) minmax(230px, 1.25fr) 190px;
	grid-template-rows: auto auto;
	column-gap: 22px;
	padding: 18px 20px;
}

.models-grid.list-view .model-card-header {
	grid-column: 1;
	grid-row: 1;
}

.models-grid.list-view .model-id-row {
	grid-column: 1;
	grid-row: 2;
	margin-top: 9px;
}

.models-grid.list-view .model-description {
	grid-column: 2;
	grid-row: 1;
	min-height: 0;
	margin: 1px 0 0;
	-webkit-line-clamp: 2;
}

.models-grid.list-view .model-specs {
	display: none;
}

.models-grid.list-view .model-card-footer {
	grid-column: 3;
	grid-row: 1 / span 2;
	align-self: stretch;
	flex-direction: column;
	justify-content: center;
	margin: 0;
	padding: 0 0 0 20px;
	border-top: 0;
	border-left: 1px solid #edf1ef;
}

.models-grid.list-view .detail-button {
	width: 100%;
}

.skeleton-card {
	pointer-events: none;
}

.skeleton {
	border-radius: 8px;
	background: linear-gradient(90deg, #f0f4f2 25%, #e3ebe7 50%, #f0f4f2 75%);
	background-size: 200% 100%;
	animation: shimmer 1.5s infinite;
}

.skeleton-heading { width: 72%; height: 42px; }
.skeleton-id { width: 58%; height: 25px; margin-top: 16px; }
.skeleton-line { width: 100%; height: 12px; margin-top: 20px; }
.skeleton-line.short { width: 72%; margin-top: 9px; }
.skeleton-footer { height: 82px; margin-top: auto; }

@keyframes shimmer {
	from { background-position: 200% 0; }
	to { background-position: -200% 0; }
}

.catalog-state {
	display: flex;
	min-height: 420px;
	align-items: center;
	justify-content: center;
	flex-direction: column;
	padding: 40px;
	border: 1px dashed #cedbd6;
	border-radius: 20px;
	background: rgba(255, 255, 255, 0.56);
	text-align: center;
}

.state-icon {
	display: grid;
	width: 58px;
	height: 58px;
	place-items: center;
	border-radius: 18px;
	background: #e8f3ee;
	color: var(--green);
}

.state-icon.error {
	background: #fff3e8;
	color: #c76719;
}

.catalog-state h3 {
	margin: 18px 0 5px;
	font-size: 18px;
}

.catalog-state p {
	margin: 0 0 20px;
	color: var(--muted);
	font-size: 12px;
}

.pagination-row {
	justify-content: space-between;
	gap: 20px;
	margin-top: 28px;
	padding-top: 22px;
	border-top: 1px solid var(--line);
}

.pagination-row > span {
	color: #64748b;
	font-size: 11px;
	font-weight: 650;
}

.pagination-row :deep(.n-pagination-item) {
	border-radius: 9px;
}

.bottom-cta {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 40px;
	margin-bottom: 82px;
	padding: 42px 46px;
	border-radius: 26px;
	background: #10231f;
	box-shadow: 0 28px 70px rgba(16, 35, 31, 0.16);
	color: #fff;
}

.bottom-cta-copy {
	gap: 20px;
}

.cta-icon {
	display: grid;
	width: 58px;
	height: 58px;
	flex: 0 0 auto;
	place-items: center;
	border-radius: 17px;
	background: var(--lime);
	color: var(--ink);
}

.bottom-cta-copy span:not(.cta-icon) {
	color: #8da29b;
	font-size: 9px;
	font-weight: 850;
	letter-spacing: 0.16em;
}

.bottom-cta h2 {
	margin: 5px 0 4px;
	font-size: clamp(23px, 3vw, 34px);
	font-weight: 800;
	letter-spacing: -0.035em;
}

.bottom-cta p {
	margin: 0;
	color: #9bb0a9;
	font-size: 12px;
}

.bottom-cta-actions {
	flex: 0 0 auto;
	gap: 10px;
}

.light-button,
.ghost-button {
	min-height: 47px;
	padding: 0 18px;
	border-radius: 13px;
	font-size: 12px;
	font-weight: 750;
}

.light-button {
	background: var(--lime);
	color: var(--ink);
}

.ghost-button {
	border: 1px solid rgba(255, 255, 255, 0.16);
	background: rgba(255, 255, 255, 0.05);
	color: #e3ece9;
}

.ghost-button:hover {
	border-color: rgba(255, 255, 255, 0.3);
	background: rgba(255, 255, 255, 0.09);
}

.public-footer {
	border-top: 1px solid var(--line);
	background: #f0f5f2;
}

.footer-inner {
	min-height: 82px;
	justify-content: space-between;
	gap: 24px;
	color: #788580;
	font-size: 10px;
}

.footer-brand {
	gap: 8px;
	color: var(--ink);
	font-size: 12px;
	font-weight: 800;
}

.footer-brand img {
	width: 24px;
	height: 24px;
	border-radius: 7px;
}

.footer-inner p {
	margin: 0;
}

.detail-modal {
	--model-accent: #0f766e;
	--model-soft: #ecfdf5;
	position: relative;
	width: min(900px, calc(100vw - 32px));
	max-height: min(820px, calc(100vh - 40px));
	overflow: auto;
	border: 1px solid rgba(255, 255, 255, 0.84);
	border-radius: 24px;
	background: #fff;
	box-shadow: 0 30px 100px rgba(8, 28, 24, 0.3);
}

.modal-close {
	position: absolute;
	top: 18px;
	right: 18px;
	z-index: 2;
	display: grid;
	width: 34px;
	height: 34px;
	place-items: center;
	border: 1px solid rgba(255, 255, 255, 0.18);
	border-radius: 10px;
	background: rgba(255, 255, 255, 0.08);
	color: #fff;
	cursor: pointer;
}

.modal-close:hover {
	background: rgba(255, 255, 255, 0.15);
}

.detail-hero {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 20px;
	padding: 34px 58px 31px 32px;
	background: var(--ink);
	color: #fff;
}

.detail-identity {
	gap: 16px;
	min-width: 0;
}

.detail-model-mark {
	display: grid;
	width: 56px;
	height: 56px;
	flex: 0 0 auto;
	place-items: center;
	border-radius: 17px;
	background: var(--model-soft);
	color: var(--model-accent);
}

.detail-provider {
	color: #89a099;
	font-size: 11px;
	font-weight: 600;
	letter-spacing: 0.1em;
	text-transform: uppercase;
}

.detail-identity h2 {
	margin: 4px 0 6px;
	font-size: 25px;
	font-weight: 700;
	letter-spacing: -0.035em;
}

.detail-model-id {
	display: inline-flex;
	max-width: 100%;
	align-items: center;
	gap: 6px;
	padding: 5px 8px;
	border: 1px solid rgba(255, 255, 255, 0.11);
	border-radius: 7px;
	background: rgba(255, 255, 255, 0.06);
	color: #b8c8c3;
	cursor: pointer;
}

.detail-model-id code {
	overflow: hidden;
	font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	font-size: 11px;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.detail-category {
	padding: 7px 10px;
	border-radius: 9px;
	background: var(--model-soft);
	color: var(--model-accent);
	font-size: 11px;
	font-weight: 700;
	white-space: nowrap;
}

.detail-loading {
	display: flex;
	min-height: 360px;
	align-items: center;
	justify-content: center;
	flex-direction: column;
	gap: 12px;
	color: #64748b;
	font-size: 12px;
}

.detail-body {
	display: grid;
	grid-template-columns: minmax(0, 1fr) 290px;
	gap: 30px;
	padding: 30px 32px 34px;
}

.detail-main-column,
.detail-side-column {
	display: flex;
	flex-direction: column;
	gap: 24px;
}

.detail-section-label,
.detail-spec-title {
	display: block;
	margin-bottom: 10px;
	color: #64748b;
	font-size: 10px;
	font-weight: 600;
	letter-spacing: 0.11em;
	text-transform: uppercase;
}

.detail-description {
	margin: 0;
	color: #56645f;
	font-size: 14px;
	line-height: 1.85;
}

.detail-capabilities,
.detail-tags {
	display: flex;
	flex-wrap: wrap;
	gap: 7px;
}

.detail-capabilities span {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	padding: 8px 10px;
	border: 1px solid #e0e8e5;
	border-radius: 10px;
	background: #f8faf9;
	color: #50605a;
	font-size: 11px;
	font-weight: 500;
}

.detail-capabilities svg {
	color: var(--model-accent);
}

.detail-tags span {
	padding: 6px 9px;
	border-radius: 8px;
	background: var(--model-soft);
	color: var(--model-accent);
	font-size: 10px;
	font-weight: 600;
}

.detail-spec-card,
.detail-spec-row,
.detail-price-row {
	display: flex;
	flex-direction: column;
	align-items: flex-start;
	gap: 4px;
	padding: 11px 0;
	border-bottom: 1px solid #e7ecea;
}

.detail-spec-row:last-child,
.detail-price-row:last-child {
	padding-bottom: 0;
	border-bottom: 0;
}

.detail-spec-row > span {
	display: inline-flex;
	align-items: center;
	gap: 7px;
	color: #64748b;
	font-size: 10px;
}

.detail-spec-row > span svg {
	color: var(--model-accent);
}

.detail-spec-row strong {
	font-size: 13px;
	font-weight: 700;
}

.detail-price-row > div {
	display: flex;
	flex-direction: column;
	gap: 2px;
}

.detail-price-row span {
	color: #586660;
	font-size: 11px;
	font-weight: 600;
}

.detail-price-row small {
	color: #64748b;
	font-size: 9px;
}

.detail-price-row strong {
	color: var(--model-accent);
	font-size: 17px;
	font-weight: 750;
}

.detail-footer {
	justify-content: space-between;
	gap: 20px;
	padding: 18px 32px;
	border-top: 1px solid #e6ece9;
	background: #f7faf8;
}

.detail-footer p {
	display: inline-flex;
	align-items: center;
	gap: 7px;
	margin: 0;
	color: #64748b;
	font-size: 10px;
}

.detail-footer .primary-button {
	min-height: 42px;
	padding: 0 16px;
	font-size: 12px;
}

@media (max-width: 1100px) {

	.models-grid {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	.models-grid.list-view .model-card:not(.skeleton-card) {
		grid-template-columns: minmax(190px, 0.9fr) minmax(210px, 1.1fr) 170px;
	}
}

@media (max-width: 900px) {
	.header-nav {
		display: none;
	}

	.catalog-layout {
		grid-template-columns: 1fr;
	}

	.category-panel {
		position: static;
	}

	.category-panel-title,
	.category-help {
		display: none;
	}

	.category-list {
		flex-direction: row;
		overflow-x: auto;
		padding-bottom: 6px;
	}

	.category-button {
		width: auto;
		min-width: max-content;
		grid-template-columns: 32px auto;
		padding: 8px 11px 8px 8px;
	}

	.category-button small,
	.category-chevron {
		display: none;
	}

	.category-icon {
		width: 31px;
		height: 31px;
	}

	.models-grid.list-view .model-card:not(.skeleton-card) {
		display: flex;
		min-height: 336px;
		padding: 19px;
	}

	.models-grid.list-view .model-card-footer {
		width: 100%;
		flex-direction: row;
		justify-content: space-between;
		margin-top: 11px;
		padding: 12px 0 0;
		border-top: 1px solid #edf1ef;
		border-left: 0;
	}

	.models-grid.list-view .model-specs {
		display: grid;
	}

	.bottom-cta {
		align-items: flex-start;
		flex-direction: column;
	}

	.detail-body {
		grid-template-columns: 1fr;
	}
}

@media (max-width: 680px) {
	.header-inner,
	.catalog-section,
	.bottom-cta,
	.footer-inner {
		width: min(100% - 28px, 1440px);
	}

	.header-inner {
		height: 68px;
	}

	.brand-copy small,
	.text-link {
		display: none;
	}

	.header-actions {
		gap: 0;
	}

	.header-cta {
		min-height: 38px;
		padding: 0 13px;
		font-size: 11px;
	}

	.primary-button,
	.secondary-button {
		width: 100%;
	}

	.catalog-section {
		padding: 62px 0 74px;
	}

	.catalog-heading {
		align-items: flex-start;
		margin-bottom: 26px;
	}

	.catalog-toolbar {
		align-items: stretch;
		flex-direction: column;
	}

	.search-box {
		width: 100%;
	}

	.toolbar-meta {
		justify-content: space-between;
	}

	.models-grid,
	.models-grid.list-view {
		grid-template-columns: 1fr;
	}

	.model-card {
		min-height: 336px;
	}

	.pagination-row {
		align-items: flex-start;
		flex-direction: column;
		overflow-x: auto;
	}

	.bottom-cta {
		margin-bottom: 56px;
		padding: 30px 24px;
		border-radius: 20px;
	}

	.bottom-cta-copy {
		align-items: flex-start;
		flex-direction: column;
	}

	.bottom-cta-actions {
		width: 100%;
		align-items: stretch;
		flex-direction: column;
	}

	.footer-inner {
		min-height: 112px;
		align-items: flex-start;
		justify-content: center;
		flex-direction: column;
		gap: 5px;
	}

	.footer-inner p {
		display: none;
	}

	.detail-modal {
		width: calc(100vw - 18px);
		max-height: calc(100vh - 18px);
		border-radius: 19px;
	}

	.detail-hero {
		align-items: flex-start;
		flex-direction: column;
		padding: 28px 50px 25px 22px;
	}

	.detail-model-mark {
		width: 48px;
		height: 48px;
	}

	.detail-identity h2 {
		font-size: 20px;
	}

	.detail-body {
		gap: 24px;
		padding: 24px 22px;
	}

	.detail-footer {
		align-items: stretch;
		flex-direction: column;
		padding: 16px 22px 20px;
	}

	.detail-footer .primary-button {
		width: 100%;
	}
}

@media (prefers-reduced-motion: reduce) {
	.header-cta,
	.primary-button,
	.secondary-button,
	.light-button,
	.ghost-button,
	.model-card,
	.detail-button {
		transition: none;
	}

	.skeleton {
		animation: none;
	}
}

/* ================================================
   Landing page visual language
   与落地页统一：青绿 mesh、液态玻璃、轻阴影、深色代码卡
   ================================================ */
.marketplace-page {
	--ink: #111827;
	--muted: #64748b;
	--line: rgba(255, 255, 255, 0.82);
	--paper: #f6f8fc;
	--surface: rgba(255, 255, 255, 0.72);
	--green: #14b8a6;
	--green-dark: #0d9488;
	--lime: #5eead4;
	-webkit-font-smoothing: antialiased;
	-moz-osx-font-smoothing: grayscale;
	text-rendering: optimizeLegibility;
	position: relative;
	overflow: hidden;
	background:
		radial-gradient(at 12% 8%, rgba(20, 184, 166, 0.1) 0, transparent 36%),
		radial-gradient(at 72% 0%, rgba(6, 182, 212, 0.08) 0, transparent 40%),
		radial-gradient(at 95% 70%, rgba(20, 184, 166, 0.06) 0, transparent 38%),
		radial-gradient(at 45% 100%, rgba(99, 102, 241, 0.04) 0, transparent 42%),
		#f6f8fc;
}

.marketplace-page::before {
	position: absolute;
	inset: 0;
	background-image:
		linear-gradient(rgba(123, 143, 245, 0.04) 1px, transparent 1px),
		linear-gradient(90deg, rgba(123, 143, 245, 0.04) 1px, transparent 1px);
	background-size: 64px 64px;
	content: '';
	pointer-events: none;
}

.marketplace-page > * {
	position: relative;
	z-index: 1;
}

.public-header {
	position: relative;
	top: auto;
	border-bottom: 0;
	background: transparent;
	backdrop-filter: none;
}

.header-inner {
	height: 64px;
	padding: 0 4px;
}

.brand-link {
	gap: 10px;
}

.brand-mark {
	width: 32px;
	height: 32px;
	border: 0;
	border-radius: 8px;
	background: transparent;
	box-shadow: none;
}

.brand-mark img {
	width: 32px;
	height: 32px;
	border-radius: 8px;
}

.brand-copy strong {
	font-size: 18px;
	font-weight: 700;
	letter-spacing: -0.025em;
}

.brand-copy small {
	display: none;
}

.header-nav {
	gap: 6px;
	padding: 4px;
	border: 1px solid rgba(255, 255, 255, 0.6);
	border-radius: 14px;
	background: rgba(241, 245, 249, 0.58);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.75);
	backdrop-filter: blur(12px);
}

.header-nav a {
	padding: 7px 14px;
	border-radius: 10px;
	color: #64748b;
	font-size: 13px;
	font-weight: 500;
}

.header-nav a.is-active,
.header-nav a:hover {
	background: rgba(255, 255, 255, 0.9);
	color: #0f766e;
	box-shadow: 0 2px 8px rgba(76, 91, 142, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.94);
}

.header-nav a.is-active::after {
	display: none;
}

.text-link {
	color: #64748b;
	font-weight: 500;
}

.header-cta {
	min-height: 38px;
	padding: 0 15px;
	border: 1px solid rgba(255, 255, 255, 0.78);
	border-radius: 10px;
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	box-shadow: 0 6px 18px rgba(20, 184, 166, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.22);
	font-size: 13px;
	font-weight: 600;
}

.header-cta:hover,
.primary-button:hover,
.light-button:hover {
	box-shadow: 0 10px 26px rgba(20, 184, 166, 0.28), inset 0 1px 0 rgba(255, 255, 255, 0.25);
}

.primary-button,
.secondary-button {
	min-height: 44px;
	padding: 0 18px;
	border-radius: 11px;
	font-size: 13px;
	font-weight: 600;
}

.primary-button {
	border: 1px solid rgba(255, 255, 255, 0.48);
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	box-shadow: 0 7px 20px rgba(20, 184, 166, 0.24), inset 0 1px 0 rgba(255, 255, 255, 0.22);
}

.secondary-button {
	border: 1px solid rgba(255, 255, 255, 0.78);
	background: rgba(255, 255, 255, 0.62);
	box-shadow: 0 5px 16px rgba(76, 91, 142, 0.07), inset 0 1px 0 rgba(255, 255, 255, 0.9);
	color: #475569;
	backdrop-filter: blur(14px);
}

.secondary-button:hover {
	border-color: rgba(255, 255, 255, 0.96);
	background: rgba(255, 255, 255, 0.9);
}

.catalog-section {
	padding: 42px 0 84px;
}

.catalog-heading {
	align-items: center;
	flex-direction: column;
	margin-bottom: 26px;
	text-align: center;
}

.section-kicker {
	padding: 5px 11px;
	border: 1px solid rgba(153, 246, 228, 0.68);
	border-radius: 999px;
	background: rgba(240, 253, 250, 0.68);
	color: #0d9488;
	font-size: 10px;
	font-weight: 600;
	letter-spacing: 0.08em;
}

.catalog-heading h2 {
	margin: 12px 0 6px;
	color: #111827;
	font-size: clamp(2rem, 4vw, 2.75rem);
	font-weight: 750;
	letter-spacing: -0.035em;
}

.catalog-heading p {
	color: #64748b;
	font-size: 13px;
}

.catalog-layout {
	grid-template-columns: 220px minmax(0, 1fr);
	gap: 24px;
	padding: 24px;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 24px;
	background: rgba(255, 255, 255, 0.64);
	box-shadow: 0 18px 55px rgba(76, 91, 142, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.9);
	backdrop-filter: blur(28px) saturate(1.16);
}

.category-panel {
	top: 24px;
}

.category-panel-title {
	color: #64748b;
	font-weight: 600;
}

.category-button {
	border-radius: 11px;
	color: #64748b;
}

.category-button:hover {
	background: rgba(255, 255, 255, 0.6);
}

.category-button.active {
	border-color: rgba(255, 255, 255, 0.9);
	background: rgba(255, 255, 255, 0.86);
	box-shadow: 0 5px 18px rgba(76, 91, 142, 0.09), inset 0 1px 0 rgba(255, 255, 255, 0.96);
}

.category-icon,
.model-mark {
	border: 1px solid rgba(255, 255, 255, 0.72);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.category-button strong {
	color: #475569;
	font-weight: 600;
}

.category-button.active strong {
	color: #0f766e;
}

.category-help {
	border: 1px solid rgba(255, 255, 255, 0.8);
	background: rgba(240, 253, 250, 0.68);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.82);
}

.category-help > span {
	background: rgba(204, 251, 241, 0.78);
	color: #0d9488;
}

.search-box input {
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 12px;
	background: rgba(255, 255, 255, 0.82);
	box-shadow: 0 6px 20px rgba(76, 91, 142, 0.07), inset 0 1px 0 rgba(255, 255, 255, 0.96);
}

.search-box input:focus {
	border-color: rgba(20, 184, 166, 0.5);
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.1), 0 8px 24px rgba(76, 91, 142, 0.09);
}

.clear-search,
.view-switch {
	background: rgba(241, 245, 249, 0.82);
}

.view-switch {
	border-color: rgba(255, 255, 255, 0.75);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.view-switch button.active {
	background: #fff;
	color: #0d9488;
	box-shadow: 0 2px 8px rgba(76, 91, 142, 0.1);
}

.model-card {
	min-height: 336px;
	border: 1px solid rgba(255, 255, 255, 0.9);
	border-radius: 16px;
	background: rgba(255, 255, 255, 0.88);
	box-shadow: 0 10px 30px rgba(76, 91, 142, 0.075), inset 0 1px 0 rgba(255, 255, 255, 0.98);
}

.model-card:hover {
	border-color: rgba(255, 255, 255, 1);
	box-shadow: 0 18px 42px rgba(76, 91, 142, 0.14), inset 0 1px 0 #fff;
}

.model-card-accent {
	right: 19px;
	left: 19px;
	height: 3px;
	background: linear-gradient(90deg, #14b8a6, #06b6d4);
	opacity: 0.72;
}

.model-provider,
.model-id-row code,
.model-description,
.model-specs span,
.price-pair span,
.price-pair small {
	color: #64748b;
}

.model-title-group h3,
.model-specs strong,
.price-pair strong {
	color: #334155;
}

.model-id-row,
.model-specs > div {
	border-color: #f1f5f9;
	background: #f8fafc;
}

.model-card-footer {
	border-top-color: #f1f5f9;
}

.detail-button {
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	box-shadow: 0 5px 14px rgba(20, 184, 166, 0.2);
}

.detail-button:hover {
	background: linear-gradient(135deg, #0d9488, #0f766e);
}

.catalog-state {
	border-color: rgba(255, 255, 255, 0.9);
	background: rgba(255, 255, 255, 0.58);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.state-icon {
	background: #f0fdfa;
	color: #0d9488;
}

.pagination-row {
	border-top-color: rgba(226, 232, 240, 0.78);
}

.bottom-cta {
	padding: 34px 38px;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 24px;
	background: rgba(255, 255, 255, 0.7);
	box-shadow: 0 18px 55px rgba(76, 91, 142, 0.11), inset 0 1px 0 rgba(255, 255, 255, 0.92);
	color: #111827;
	backdrop-filter: blur(28px) saturate(1.18);
}

.cta-icon {
	background: linear-gradient(135deg, #14b8a6, #06b6d4);
	box-shadow: 0 10px 26px rgba(20, 184, 166, 0.23);
	color: #fff;
}

.bottom-cta-copy span:not(.cta-icon) {
	color: #0d9488;
}

.bottom-cta h2 {
	color: #111827;
}

.bottom-cta p {
	color: #64748b;
}

.light-button {
	background: linear-gradient(135deg, #14b8a6, #0d9488);
	box-shadow: 0 7px 20px rgba(20, 184, 166, 0.23);
	color: #fff;
}

.ghost-button {
	border: 1px solid rgba(255, 255, 255, 0.88);
	background: rgba(255, 255, 255, 0.62);
	box-shadow: 0 5px 16px rgba(76, 91, 142, 0.07), inset 0 1px 0 rgba(255, 255, 255, 0.92);
	color: #475569;
}

.ghost-button:hover {
	border-color: #fff;
	background: #fff;
}

.public-footer {
	border-top: 0;
	background: transparent;
}

.detail-modal {
	border: 1px solid rgba(255, 255, 255, 0.9);
	background: rgba(255, 255, 255, 0.9);
	box-shadow: 0 24px 80px rgba(76, 91, 142, 0.24), inset 0 1px 0 #fff;
	backdrop-filter: blur(28px) saturate(1.16);
}

.detail-hero {
	background:
		radial-gradient(at 80% 0%, rgba(20, 184, 166, 0.17), transparent 42%),
		#0f172a;
}

.detail-spec-card {
	border-color: rgba(255, 255, 255, 0.9);
	background: rgba(248, 250, 252, 0.82);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.94);
}

.detail-price-row strong {
	color: #0d9488;
}

.detail-footer {
	border-top-color: rgba(226, 232, 240, 0.8);
	background: rgba(248, 250, 252, 0.82);
}

@media (max-width: 900px) {

	.catalog-layout {
		padding: 20px;
	}
}

@media (max-width: 680px) {
	.header-inner,
	.catalog-section,
	.bottom-cta,
	.footer-inner {
		width: min(100% - 32px, 1440px);
	}

	.header-inner {
		height: 64px;
	}

	.catalog-section {
		padding-top: 34px;
	}

	.catalog-heading h2 {
		font-size: 1.85rem;
	}

	.catalog-layout {
		padding: 14px;
		border-radius: 20px;
	}

	.category-list {
		margin: 0 -4px;
	}

	.model-card {
		min-height: 336px;
	}

	.bottom-cta {
		padding: 28px 22px;
	}
}

/* 模型目录可读性：正文与数据字号统一提升 */
.catalog-heading p {
	font-size: 15px;
}

.category-panel-title,
.active-filter-label,
.pagination-row > span {
	font-size: 12px;
}

.category-button strong {
	font-size: 14px;
}

.category-button small {
	font-size: 12px;
}

.category-help strong {
	font-size: 13px;
}

.category-help p {
	font-size: 12px;
}

.search-box input {
	font-size: 14px;
}

.model-provider {
	font-size: 11px;
}

.model-title-group h3 {
	font-size: 17px;
}

.model-id-row code {
	font-size: 12px;
}

.model-description {
	min-height: 68px;
	font-size: 13px;
	line-height: 1.7;
}

.model-specs span {
	font-size: 11px;
}

.model-specs strong {
	font-size: 13px;
}

.price-pair span,
.price-pair small {
	font-size: 11px;
}

.price-pair strong {
	font-size: 14px;
}

.detail-button {
	min-height: 38px;
	font-size: 12px;
}

@media (max-width: 680px) {
	.catalog-heading p {
		font-size: 13px;
	}

	.model-title-group h3 {
		font-size: 16px;
	}

	.model-description {
		font-size: 12px;
	}
}
</style>
