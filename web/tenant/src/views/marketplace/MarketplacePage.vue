<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import BasePagination from '@/components/common/BasePagination.vue'
import ModelCard from './components/ModelCard.vue'
import ModelDetailDrawer from './components/ModelDetailDrawer.vue'
import { getMarketplaceModelDetail, getMarketplaceModels, type MarketplaceModel } from '@/api/marketplace'
import { categoryMetaList } from './marketplaceMeta'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useSeo } from '@/composables/useSeo'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { toast } from '@/utils/toast'

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
const categories = categoryMetaList

const keyword = ref('')
const selectedCategory = ref<string | null>(null)
const loading = ref(false)
const loadError = ref(false)
const modelList = ref<MarketplaceModel[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(12)
const catalogSection = ref<HTMLElement | null>(null)
const searchInputElement = ref<HTMLInputElement | null>(null)
const detailVisible = ref(false)
const detailLoading = ref(false)
const selectedModel = ref<MarketplaceModel | null>(null)

let searchTimer: ReturnType<typeof setTimeout> | null = null
let listRequestId = 0
let detailRequestId = 0

const hasActiveFilters = computed(() => keyword.value.trim() !== '' || selectedCategory.value !== null)

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

// 页码 / 每页条数变化（BasePagination 已先把 v-model 更新到位）
function handlePageChange(): void {
	loadModels()
	catalogSection.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// 详情抽屉：先用列表数据渲染，再异步补全详情；请求竞态用自增序号兜底
async function openDetail(model: MarketplaceModel): Promise<void> {
	selectedModel.value = model
	detailVisible.value = true
	detailLoading.value = true
	const requestId = ++detailRequestId

	try {
		const detail = await getMarketplaceModelDetail(model.model_id)
		// 抽屉已关闭或请求已被更新时丢弃迟到响应，避免覆盖新选中的模型
		if (requestId === detailRequestId && detailVisible.value) selectedModel.value = detail
	} catch {
		// 列表数据足够展示基础详情，接口失败时保持已有内容。
	} finally {
		if (requestId === detailRequestId) detailLoading.value = false
	}
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

<template>
	<div class="relative min-h-screen overflow-x-clip">
		<!-- 装饰层：光球 + 网格纹理，只做背景加强，不参与布局 -->
		<div aria-hidden="true" class="pointer-events-none absolute inset-0 overflow-hidden">
			<div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/20 blur-3xl"></div>
			<div class="absolute -bottom-40 -left-40 h-80 w-80 rounded-full bg-primary-500/15 blur-3xl"></div>
			<div class="absolute left-1/2 top-1/3 h-96 w-96 -translate-x-1/2 rounded-full bg-primary-300/10 blur-3xl"></div>
			<div
				class="absolute inset-0"
				style="background-image: linear-gradient(rgba(20, 184, 166, 0.035) 1px, transparent 1px), linear-gradient(90deg, rgba(20, 184, 166, 0.035) 1px, transparent 1px); background-size: 64px 64px"
			></div>
		</div>

		<!-- 品牌栏：与落地页语言统一 -->
		<header class="glass sticky top-0 z-40 border-b border-white/40">
			<div class="flex h-16 items-center justify-between px-4 sm:px-6 lg:px-8">
				<router-link :to="{ name: 'TenantHome' }" class="flex items-center gap-2.5" aria-label="返回首页">
					<img src="/favicon.png" :alt="siteName" class="h-8 w-8 rounded-lg" />
					<span class="text-lg font-bold tracking-tight text-gray-900">{{ siteName }}</span>
					<span class="ml-1 hidden rounded-full border border-primary-200/70 bg-primary-50/80 px-2.5 py-0.5 text-[11px] font-semibold text-primary-700 sm:inline-block">模型广场</span>
				</router-link>
				<div class="flex items-center gap-2">
					<router-link v-if="!isLoggedIn" :to="{ name: 'TenantLogin' }" class="btn btn-ghost btn-sm">登录</router-link>
					<router-link :to="primaryActionRoute" class="btn btn-primary btn-sm">
						{{ primaryActionLabel }}
						<Icon name="arrowRight" size="sm" />
					</router-link>
				</div>
			</div>
		</header>

		<main class="relative">
			<!-- 主体：左侧悬浮筛选面板 + 右侧卡片目录 -->
			<section ref="catalogSection" class="scroll-mt-24 px-4 pb-10 pt-6 sm:px-6 sm:pt-8 lg:px-8">
				<div class="flex flex-col gap-5 lg:flex-row lg:items-start lg:gap-6">
					<!-- 左侧悬浮筛选面板：桌面端吸附跟随 -->
					<aside class="shrink-0 lg:sticky lg:top-24 lg:w-72">
						<div class="glass rounded-[1.75rem] p-4 shadow-glass lg:max-h-[calc(100dvh-7.5rem)] lg:overflow-y-auto">
							<span class="mb-3 block text-xs font-semibold uppercase tracking-wider text-gray-400">筛选</span>

							<!-- 搜索 -->
							<div class="flex h-11 items-center gap-2.5 rounded-xl border border-gray-200/70 bg-white/80 px-3.5 transition-all duration-200 focus-within:border-primary-400/70 focus-within:ring-2 focus-within:ring-primary-500/20">
								<Icon name="search" size="sm" class="shrink-0 text-gray-400" />
								<input
									ref="searchInputElement"
									v-model="keyword"
									type="search"
									aria-label="搜索模型"
									placeholder="搜索名称、ID 或描述"
									class="h-full min-w-0 flex-1 border-0 bg-transparent text-sm text-gray-900 outline-none placeholder:text-gray-400"
									@input="scheduleSearch"
									@keydown.enter.prevent="runSearchNow"
								/>
								<button
									v-if="keyword"
									type="button"
									aria-label="清除搜索"
									class="grid h-6 w-6 shrink-0 place-items-center rounded-full text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600"
									@click="clearSearch"
								>
									<Icon name="x" size="xs" />
								</button>
							</div>

							<div class="my-4 h-px bg-gray-200/60"></div>

							<!-- 分类：移动端横向 chips，桌面端菜单列表 -->
							<span class="mb-3 hidden text-xs font-semibold uppercase tracking-wider text-gray-400 lg:block">模型类型</span>
							<nav class="flex flex-wrap gap-2 lg:flex-col lg:gap-1" aria-label="按模型类别筛选">
								<button
									v-for="cat in categories"
									:key="cat.key"
									type="button"
									:title="cat.description"
									class="inline-flex items-center gap-2 rounded-full border px-3.5 py-2 text-sm font-medium transition-all duration-200 active:scale-[0.98] lg:w-full lg:rounded-xl lg:px-3.5 lg:py-2.5"
									:class="
										selectedCategory === cat.value
											? 'border-transparent text-white shadow-glow lg:font-semibold'
											: 'border-gray-200/70 bg-white/70 text-gray-600 hover:border-primary-200 hover:text-primary-700 lg:border-transparent lg:bg-transparent lg:hover:bg-primary-500/5 lg:hover:text-primary-700'
									"
									:style="
										selectedCategory === cat.value
											? { background: `linear-gradient(135deg, ${cat.from}, ${cat.to})`, boxShadow: `0 10px 30px rgba(${cat.glow}, 0.35)` }
											: undefined
									"
									@click="selectCategory(cat.value)"
								>
									<Icon :name="cat.icon" size="sm" class="shrink-0" />
									{{ cat.shortLabel }}
									<Icon v-if="selectedCategory === cat.value" name="check" size="xs" class="ml-auto hidden shrink-0 lg:block" />
								</button>
							</nav>

							<!-- 重置筛选：仅在存在筛选条件时出现 -->
							<button v-if="hasActiveFilters" type="button" class="btn btn-secondary btn-sm mt-4 w-full" @click="resetFilters">
								<Icon name="refresh" size="sm" />重置筛选
							</button>
						</div>
					</aside>

					<!-- 右侧目录 -->
					<div class="min-w-0 flex-1">
						<div class="flex items-center justify-between pb-4">
							<p class="text-sm text-gray-500">
								共 <strong class="font-semibold tabular-nums text-gray-900">{{ total }}</strong> 个模型
								<template v-if="keyword"> · 搜索“{{ keyword.trim() }}”</template>
							</p>
						</div>

						<!-- 加载骨架 -->
						<div v-if="loading" class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4" aria-label="正在加载模型">
							<div v-for="i in pageSize" :key="i" class="rounded-[20px] border border-white/80 bg-white/85 p-5 shadow-card">
								<div class="flex items-start gap-4">
									<div class="skeleton h-14 w-14 rounded-2xl"></div>
									<div class="flex-1 space-y-2 pt-1">
										<div class="skeleton h-4 w-3/4 rounded"></div>
										<div class="skeleton h-3 w-1/2 rounded"></div>
									</div>
								</div>
								<div class="skeleton mt-4 h-3 w-full rounded"></div>
								<div class="skeleton mt-2 h-3 w-2/3 rounded"></div>
								<div class="mt-4 flex gap-2">
									<div class="skeleton h-12 flex-1 rounded-xl"></div>
									<div class="skeleton h-12 flex-1 rounded-xl"></div>
								</div>
								<div class="skeleton mt-4 h-8 w-full rounded-xl"></div>
							</div>
						</div>

						<!-- 错误态 -->
						<div v-else-if="loadError" class="empty-state">
							<Icon name="exclamationTriangle" size="xl" class="empty-state-icon" />
							<p class="empty-state-title">模型列表加载失败</p>
							<p class="empty-state-description">网络可能暂时不可用，请稍后重试。</p>
							<button type="button" class="btn btn-secondary btn-sm mt-4" @click="loadModels">
								<Icon name="refresh" size="sm" />重新加载
							</button>
						</div>

						<!-- 空态 -->
						<div v-else-if="modelList.length === 0" class="empty-state">
							<Icon name="search" size="xl" class="empty-state-icon" />
							<p class="empty-state-title">没有找到匹配的模型</p>
							<p class="empty-state-description">换一个关键词，或者清除当前分类后再试。</p>
							<button type="button" class="btn btn-secondary btn-sm mt-4" @click="resetFilters">
								<Icon name="refresh" size="sm" />重置筛选
							</button>
						</div>

						<!-- 卡片网格：key 携带分类与页码，翻页 / 切分类时重放入场动画 -->
						<div v-else :key="`${selectedCategory ?? 'all'}-${currentPage}`" class="grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
							<ModelCard
								v-for="(model, index) in modelList"
								:key="model.model_id"
								:model="model"
								:index="index"
								@open="openDetail"
								@copy="copyModelId"
							/>
						</div>

						<!-- 分页 -->
						<div v-if="!loading && !loadError && total > 0" class="mt-8">
							<BasePagination
								v-model="currentPage"
								v-model:page-size="pageSize"
								:total="total"
								:page-size-options="[12, 24, 48]"
								@change="handlePageChange"
							/>
						</div>
					</div>
				</div>
			</section>

		</main>

		<footer class="relative border-t border-white/40 py-8">
			<div class="flex flex-col items-center gap-2 px-4 text-center sm:px-6 lg:px-8">
				<div class="flex items-center gap-2">
					<img src="/favicon.png" :alt="siteName" class="h-5 w-5 rounded" />
					<span class="text-sm font-semibold text-gray-700">{{ siteName }}</span>
				</div>
				<p class="text-xs text-gray-400">统一的大模型 API 接入与管理平台</p>
				<span class="text-xs text-gray-400">© {{ currentYear }} qianfree · AGPL-3.0</span>
			</div>
		</footer>

		<!-- 模型详情抽屉 -->
		<ModelDetailDrawer
			v-model:show="detailVisible"
			:model="selectedModel"
			:loading="detailLoading"
			:is-logged-in="isLoggedIn"
			:primary-action-route="primaryActionRoute"
			@copy="copyModelId"
		/>
	</div>
</template>
