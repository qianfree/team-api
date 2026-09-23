<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import BasePagination from '@/components/common/BasePagination.vue'
import ModelCard from './components/ModelCard.vue'
import ModelDetailDrawer from './components/ModelDetailDrawer.vue'
import { getMarketplaceModelDetail, getMarketplaceModels, type MarketplaceModel } from '@/api/marketplace'
import VendorLogo from '@/components/common/VendorLogo.vue'
import { categoryMetaList } from './marketplaceMeta'
import { vendorMetaList, vendorLabel } from '@/constants/vendor'
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
const selectedVendor = ref<string | null>(null)
// 当前条件下有模型的厂商（后端 facet 返回），厂商筛选项据此动态渲染
const availableVendors = ref<string[]>([])
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

const hasActiveFilters = computed(() => keyword.value.trim() !== '' || selectedCategory.value !== null || selectedVendor.value !== null)

// 厂商筛选项：只显示当前条件下实际有模型的厂商（按字典序）；已选厂商不在可用列表时保留，
// 否则筛选条件生效但按钮消失，用户无法取消选择。字典外的厂商值（历史数据）按原值追加，渲染回退头像。
const visibleVendorOptions = computed(() => {
	const available = new Set(availableVendors.value)
	if (selectedVendor.value) available.add(selectedVendor.value)
	const known = vendorMetaList
		.filter(v => available.has(v.value))
		.map(v => ({ value: v.value, label: v.label }))
	const knownValues = new Set(vendorMetaList.map(v => v.value))
	const unknown = [...available]
		.filter(v => !knownValues.has(v))
		.map(v => ({ value: v, label: vendorLabel(v) }))
	return [...known, ...unknown]
})

async function loadModels(): Promise<void> {
	const requestId = ++listRequestId
	loading.value = true
	loadError.value = false

	try {
		const response = await getMarketplaceModels({
			keyword: keyword.value.trim() || undefined,
			category: selectedCategory.value || undefined,
			vendor: selectedVendor.value || undefined,
			page: currentPage.value,
			page_size: pageSize.value,
		})
		if (requestId !== listRequestId) return
		modelList.value = response.list || []
		total.value = response.total || 0
		availableVendors.value = response.vendors || []
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

function selectVendor(vendor: string | null): void {
	if (selectedVendor.value === vendor) return
	selectedVendor.value = vendor
	currentPage.value = 1
	loadModels()
}

function resetFilters(): void {
	keyword.value = ''
	selectedCategory.value = null
	selectedVendor.value = null
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
			<div class="absolute -right-40 -top-40 h-80 w-80 rounded-full bg-primary-400/15 blur-3xl"></div>
			<div class="absolute -left-40 top-1/3 h-80 w-80 rounded-full bg-primary-500/10 blur-3xl"></div>
			<div
				class="absolute inset-0"
				style="background-image: linear-gradient(rgba(20, 184, 166, 0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(20, 184, 166, 0.03) 1px, transparent 1px); background-size: 64px 64px"
			></div>
		</div>

		<!-- 品牌栏：与落地页语言统一 -->
		<header class="glass sticky top-0 z-40 border-b border-white/40">
			<div class="mx-auto flex h-16 max-w-[92rem] items-center justify-between px-4 sm:px-6 lg:px-8">
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

		<main class="relative mx-auto w-full max-w-[92rem] px-4 pb-14 sm:px-6 lg:px-8">
			<!-- 标题区 -->
			<section class="pb-5 pt-7 sm:pb-6 sm:pt-9">
				<h1 class="text-2xl font-bold tracking-tight text-gray-900 sm:text-[28px]">模型广场</h1>
				<p class="mt-1.5 max-w-2xl text-sm leading-relaxed text-gray-500">
					浏览主流 AI 模型，比较能力、上下文窗口与 Token 价格，通过统一的 OpenAI 兼容接口接入。
				</p>
			</section>

			<!-- 筛选：搜索 + 类型 + 厂商，统一收在一张卡里 -->
			<section class="card mb-5 overflow-hidden" aria-label="模型筛选">
				<div class="flex flex-col gap-3 px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between sm:px-5">
					<div class="flex h-11 w-full items-center gap-2.5 rounded-xl border border-gray-200 bg-white px-3.5 transition-all duration-200 focus-within:border-primary-400 focus-within:ring-2 focus-within:ring-primary-500/20 sm:max-w-md">
						<Icon name="search" size="sm" class="shrink-0 text-gray-400" />
						<input
							ref="searchInputElement"
							v-model="keyword"
							type="search"
							aria-label="搜索模型"
							placeholder="搜索模型名称、ID 或描述"
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

					<div class="flex shrink-0 items-center justify-between gap-3 sm:justify-end">
						<p class="text-sm text-gray-500">
							共 <strong class="font-semibold tabular-nums text-gray-900">{{ total }}</strong> 个模型
							<template v-if="keyword.trim()"> · “{{ keyword.trim() }}”</template>
						</p>
						<button v-if="hasActiveFilters" type="button" class="btn btn-secondary btn-sm" @click="resetFilters">
							<Icon name="refresh" size="xs" />重置
						</button>
					</div>
				</div>

				<!-- 模型类型 -->
				<div class="flex items-center gap-2.5 border-t border-gray-100 px-4 py-2.5 sm:px-5">
					<span class="hidden w-8 shrink-0 text-xs font-medium text-gray-400 sm:block">类型</span>
					<nav class="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto pb-0.5" aria-label="按模型类别筛选">
						<button
							v-for="cat in categories"
							:key="cat.key"
							type="button"
							:title="cat.description"
							:aria-pressed="selectedCategory === cat.value"
							class="inline-flex shrink-0 items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-all duration-200 active:scale-[0.98]"
							:class="
								selectedCategory === cat.value
									? 'bg-primary-500 text-white shadow-sm shadow-primary-500/30'
									: 'text-gray-600 hover:bg-gray-100 hover:text-gray-900'
							"
							@click="selectCategory(cat.value)"
						>
							<Icon :name="cat.icon" size="sm" class="shrink-0" />
							{{ cat.shortLabel }}
						</button>
					</nav>
				</div>

				<!-- 研发厂商 -->
				<div v-if="visibleVendorOptions.length" class="flex items-center gap-2.5 border-t border-gray-100 px-4 py-2.5 sm:px-5">
					<span class="hidden w-8 shrink-0 text-xs font-medium text-gray-400 sm:block">厂商</span>
					<nav class="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto pb-0.5" aria-label="按厂商筛选">
						<button
							type="button"
							:aria-pressed="selectedVendor === null"
							class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-all duration-200 active:scale-[0.98]"
							:class="
								selectedVendor === null
									? 'border-primary-500 bg-primary-50 text-primary-700'
									: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-700'
							"
							@click="selectVendor(null)"
						>
							全部
						</button>
						<button
							v-for="vendor in visibleVendorOptions"
							:key="vendor.value"
							type="button"
							:aria-pressed="selectedVendor === vendor.value"
							class="inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-all duration-200 active:scale-[0.98]"
							:class="
								selectedVendor === vendor.value
									? 'border-primary-500 bg-primary-50 text-primary-700'
									: 'border-gray-200 bg-white text-gray-600 hover:border-primary-300 hover:text-primary-700'
							"
							@click="selectVendor(vendor.value)"
						>
							<VendorLogo :vendor="vendor.value" size="xs" />
							<span class="whitespace-nowrap">{{ vendor.label }}</span>
							<Icon v-if="selectedVendor === vendor.value" name="check" size="xs" class="shrink-0" />
						</button>
					</nav>
				</div>
			</section>

			<!-- 目录 -->
			<section ref="catalogSection" class="scroll-mt-24">
				<!-- 加载骨架 -->
				<div v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4" aria-label="正在加载模型">
					<div v-for="i in pageSize" :key="i" class="card p-4">
						<div class="flex items-start gap-3">
							<div class="skeleton h-7 w-7 rounded-lg"></div>
							<div class="flex-1 space-y-2 pt-1">
								<div class="skeleton h-4 w-3/4 rounded"></div>
								<div class="skeleton h-3 w-1/2 rounded"></div>
							</div>
						</div>
						<div class="skeleton mt-3 h-3 w-full rounded"></div>
						<div class="skeleton mt-2 h-3 w-2/3 rounded"></div>
						<div class="skeleton mt-3 h-3 w-1/2 rounded"></div>
						<div class="mt-5 flex items-end justify-between border-t border-gray-100 pt-3.5">
							<div class="skeleton h-6 w-24 rounded"></div>
							<div class="skeleton h-6 w-14 rounded-lg"></div>
						</div>
					</div>
				</div>

				<!-- 错误态 -->
				<div v-else-if="loadError" class="card empty-state">
					<Icon name="exclamationTriangle" size="xl" class="empty-state-icon" />
					<p class="empty-state-title">模型列表加载失败</p>
					<p class="empty-state-description">网络可能暂时不可用，请稍后重试。</p>
					<button type="button" class="btn btn-secondary btn-sm mt-4" @click="loadModels">
						<Icon name="refresh" size="sm" />重新加载
					</button>
				</div>

				<!-- 空态 -->
				<div v-else-if="modelList.length === 0" class="card empty-state">
					<Icon name="search" size="xl" class="empty-state-icon" />
					<p class="empty-state-title">没有找到匹配的模型</p>
					<p class="empty-state-description">换一个关键词，或者清除当前筛选条件后再试。</p>
					<button type="button" class="btn btn-secondary btn-sm mt-4" @click="resetFilters">
						<Icon name="refresh" size="sm" />重置筛选
					</button>
				</div>

				<!-- 卡片网格：key 携带分类与页码，翻页 / 切分类时重放入场动画 -->
				<div v-else :key="`${selectedCategory ?? 'all'}-${selectedVendor ?? 'all'}-${currentPage}`" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
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
				<div v-if="!loading && !loadError && total > 0" class="mt-7">
					<BasePagination
						v-model="currentPage"
						v-model:page-size="pageSize"
						:total="total"
						:page-size-options="[12, 24, 48]"
						@change="handlePageChange"
					/>
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
