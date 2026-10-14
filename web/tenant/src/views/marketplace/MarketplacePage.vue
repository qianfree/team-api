<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { NConfigProvider, NDrawer, NDrawerContent, darkTheme } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import BasePagination from '@/components/common/BasePagination.vue'
import FilterPanel from './components/FilterPanel.vue'
import ModelCard from './components/ModelCard.vue'
import ModelDetailDrawer from './components/ModelDetailDrawer.vue'
import ShowcaseStrip from './components/ShowcaseStrip.vue'
import {
	getMarketplaceModelDetail,
	getMarketplaceModels,
	getMarketplaceShowcase,
	type MarketplaceFacets,
	type MarketplaceModel,
} from '@/api/marketplace'
import { categoryMetaList, marketplaceSortOptions, capabilityOptionLabel } from './marketplaceMeta'
import { vendorLabel } from '@/constants/vendor'
import { useIsMobile } from '@/composables/useIsMobile'
import { usePublicSettings } from '@/composables/usePublicSettings'
import { useSeo } from '@/composables/useSeo'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import { toast } from '@/utils/toast'

const authStore = useTenantAuthStore()
const { settings: publicSettings, fetchSettings } = usePublicSettings()
// 窄屏断点取 1024（与左栏显隐断点一致），768-1024 之间的平板同样走底部 sheet
const { isMobile: isNarrow } = useIsMobile(1024)

// ── 主题：默认明亮，可切暗色，localStorage 记住选择 ──
const THEME_STORAGE_KEY = 'mp-theme'

function loadTheme(): 'light' | 'dark' {
	try {
		const saved = localStorage.getItem(THEME_STORAGE_KEY)
		return saved === 'dark' ? 'dark' : 'light'
	} catch {
		return 'light'
	}
}

const theme = ref<'light' | 'dark'>(loadTheme())
const isDark = computed(() => theme.value === 'dark')

function toggleTheme(): void {
	theme.value = isDark.value ? 'light' : 'dark'
	try {
		localStorage.setItem(THEME_STORAGE_KEY, theme.value)
	} catch {
		// 隐私模式等场景写入失败不影响本次会话
	}
}

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

// ── 筛选/排序状态（服务端分页 + 析取分面） ──
const keyword = ref('')
const category = ref<string | null>(null)
const vendor = ref<string | null>(null)
const capabilities = ref<string[]>([])
const billingMode = ref('')
const sort = ref('')

const loading = ref(false)
const loadError = ref(false)
const modelList = ref<MarketplaceModel[]>([])
const facets = ref<MarketplaceFacets | null>(null)
const total = ref(0)
const vendorCount = ref(0)
const currentPage = ref(1)
const pageSize = ref(24)

// 展示带（运营推荐位，不参与筛选）
const showcase = ref<{ recommended: MarketplaceModel[]; newest: MarketplaceModel[] }>({ recommended: [], newest: [] })

// 详情抽屉
const detailVisible = ref(false)
const detailLoading = ref(false)
const selectedModel = ref<MarketplaceModel | null>(null)

// 移动端筛选 sheet
const sheetVisible = ref(false)

let searchTimer: ReturnType<typeof setTimeout> | null = null
let listRequestId = 0
let detailRequestId = 0

const hasActiveFilters = computed(
	() =>
		keyword.value.trim() !== '' ||
		category.value !== null ||
		vendor.value !== null ||
		capabilities.value.length > 0 ||
		billingMode.value !== '',
)

const showStrips = computed(() => !hasActiveFilters.value)

// 工具栏已选条件 chips（逐个可删）
interface ActiveChip {
	key: string
	label: string
	remove: () => void
}

const activeChips = computed<ActiveChip[]>(() => {
	const chips: ActiveChip[] = []
	if (category.value) {
		const label = categoryMetaList.find(c => c.value === category.value)?.label ?? category.value
		chips.push({ key: `cat-${category.value}`, label, remove: () => (category.value = null) })
	}
	if (vendor.value) {
		chips.push({
			key: `vendor-${vendor.value}`,
			label: vendor.value === 'uncategorized' ? '未分类' : vendorLabel(vendor.value),
			remove: () => setVendor(null),
		})
	}
	for (const cap of capabilities.value) {
		chips.push({ key: `cap-${cap}`, label: capabilityOptionLabel(cap), remove: () => toggleCapability(cap) })
	}
	if (billingMode.value) {
		const label = marketplaceBillingLabel(billingMode.value)
		chips.push({ key: 'billing', label, remove: () => (billingMode.value = '') })
	}
	return chips
})

function marketplaceBillingLabel(mode: string): string {
	const found = [
		{ value: 'token', label: '按量计费' },
		{ value: 'per_request', label: '按次计费' },
		{ value: 'per_second', label: '按秒计费' },
		{ value: 'tiered', label: '阶梯计费' },
	].find(m => m.value === mode)
	return found?.label ?? mode
}

async function loadModels(): Promise<void> {
	const requestId = ++listRequestId
	loading.value = true
	loadError.value = false

	try {
		const response = await getMarketplaceModels({
			keyword: keyword.value.trim() || undefined,
			categories: category.value || undefined,
			vendor: vendor.value || undefined,
			capabilities: capabilities.value.length ? capabilities.value.join(',') : undefined,
			billing_mode: billingMode.value || undefined,
			sort: sort.value || undefined,
			page: currentPage.value,
			page_size: pageSize.value,
		})
		if (requestId !== listRequestId) return
		modelList.value = response.list || []
		total.value = response.total || 0
		facets.value = response.facets ?? null
		vendorCount.value = Object.keys(response.facets?.vendors ?? {}).length || response.vendors?.length || 0
	} catch {
		if (requestId !== listRequestId) return
		modelList.value = []
		total.value = 0
		loadError.value = true
	} finally {
		if (requestId === listRequestId) loading.value = false
	}
}

async function loadShowcase(): Promise<void> {
	try {
		const res = await getMarketplaceShowcase()
		showcase.value = { recommended: res.recommended || [], newest: res.newest || [] }
	} catch {
		// 展示带属增强内容，失败静默隐藏
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
}

function onFilterChange(): void {
	currentPage.value = 1
	loadModels()
}

// 厂商单选：点击即切换（null=全部）
function setVendor(value: string | null): void {
	vendor.value = value
	onFilterChange()
}

function toggleCapability(value: string): void {
	capabilities.value = capabilities.value.includes(value)
		? capabilities.value.filter(v => v !== value)
		: [...capabilities.value, value]
	onFilterChange()
}

function resetFilters(): void {
	keyword.value = ''
	category.value = null
	vendor.value = null
	capabilities.value = []
	billingMode.value = ''
	currentPage.value = 1
	loadModels()
}

function onSortChange(): void {
	currentPage.value = 1
	loadModels()
}

// 新上线「查看全部」→ 切最新上架排序（同时清品类便于浏览全量）
function viewAllNewest(): void {
	sort.value = 'newest'
	category.value = null
	onSortChange()
	document.getElementById('mp-catalog')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function handlePageChange(): void {
	loadModels()
	document.getElementById('mp-catalog')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// 详情抽屉：先用列表数据渲染，再异步补全详情；请求竞态用自增序号兜底
async function openDetail(model: MarketplaceModel): Promise<void> {
	selectedModel.value = model
	detailVisible.value = true
	detailLoading.value = true
	const requestId = ++detailRequestId

	try {
		const detail = await getMarketplaceModelDetail(model.model_id)
		if (requestId === detailRequestId && detailVisible.value) selectedModel.value = detail
	} catch {
		// 列表数据足够展示基础详情，接口失败时保持已有内容
	} finally {
		if (requestId === detailRequestId) detailLoading.value = false
	}
}

async function copyText(text: string, successMessage: string): Promise<void> {
	try {
		if (navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(text)
		} else {
			const textarea = document.createElement('textarea')
			textarea.value = text
			textarea.style.position = 'fixed'
			textarea.style.opacity = '0'
			document.body.appendChild(textarea)
			textarea.select()
			document.execCommand('copy')
			textarea.remove()
		}
		toast.success(successMessage)
	} catch {
		toast.warning('复制失败，请手动复制')
	}
}

// naive-ui 组件主题跟随页面主题（抽屉/分页等）
const naiveTheme = computed(() => (isDark.value ? darkTheme : null))
const naiveThemeOverrides = computed(() => ({
	common: {
		primaryColor: isDark.value ? '#2dd4bf' : '#14b8a6',
		primaryColorHover: isDark.value ? '#5eead4' : '#0d9488',
		primaryColorPressed: '#0d9488',
		primaryColorSuppl: isDark.value ? '#2dd4bf' : '#14b8a6',
		borderRadius: '8px',
	},
}))

onMounted(() => {
	authStore.loadFromStorage()
	fetchSettings()
	loadModels()
	loadShowcase()
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
	<n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides" class="mp-provider">
		<div class="mp" :data-mp-theme="theme">
			<!-- 装饰层：极淡网格 + 双光晕（落地页同族语言，低强度） -->
			<div aria-hidden="true" class="mp-deco">
				<div class="mp-deco-grid"></div>
				<div class="mp-deco-glow mp-deco-glow-a"></div>
				<div class="mp-deco-glow mp-deco-glow-b"></div>
			</div>

			<!-- 顶栏 -->
			<header class="mp-header">
				<div class="mx-auto flex h-16 w-full max-w-[100rem] items-center justify-between px-4 sm:px-6 lg:px-8">
					<router-link :to="{ name: 'TenantHome' }" class="flex items-center gap-2.5" aria-label="返回首页">
						<img src="/favicon.png" :alt="siteName" class="h-8 w-8 rounded-lg" />
						<span class="text-lg font-bold tracking-tight">{{ siteName }}</span>
						<span class="mp-header-tag">模型广场</span>
					</router-link>
					<div class="flex items-center gap-2">
						<button
							type="button"
							class="mp-theme-toggle"
							:aria-label="isDark ? '切换到明亮模式' : '切换到暗色模式'"
							:title="isDark ? '切换到明亮模式' : '切换到暗色模式'"
							@click="toggleTheme"
						>
							<Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
						</button>
						<router-link v-if="!isLoggedIn" :to="{ name: 'TenantLogin' }" class="mp-btn mp-btn-ghost mp-btn-sm">登录</router-link>
						<router-link :to="primaryActionRoute" class="mp-btn mp-btn-primary mp-btn-sm">
							{{ primaryActionLabel }}
							<Icon name="arrowRight" size="sm" />
						</router-link>
					</div>
				</div>
			</header>

			<main class="relative mx-auto w-full max-w-[100rem] px-4 pb-16 sm:px-6 lg:px-8">
				<!-- 页头带：紧凑（标题 + 统计 + 搜索一行区，让工作区尽早进入首屏） -->
				<section class="pb-5 pt-6 sm:pt-7">
					<div class="flex flex-wrap items-end justify-between gap-x-4 gap-y-1.5">
						<div class="flex flex-wrap items-baseline gap-x-3">
							<h1 class="text-[22px] font-bold tracking-tight sm:text-2xl">模型广场</h1>
							<p class="text-[13px] text-[var(--mp-text-dim)]">统一 API 调用主流大模型，按量透明计费</p>
						</div>
						<p v-if="total > 0 || vendorCount > 0" class="mp-stats">
							<strong class="font-mono tabular-nums">{{ total }}</strong> 个模型
							<span class="mp-stats-dot">·</span>
							<strong class="font-mono tabular-nums">{{ vendorCount }}</strong> 家厂商
						</p>
					</div>

					<div class="mp-search" :class="{ 'is-filled': keyword.trim() !== '' }">
						<Icon name="search" size="md" class="shrink-0 text-[var(--mp-text-mute)]" />
						<input
							v-model="keyword"
							type="search"
							aria-label="搜索模型"
							placeholder="搜索模型名称、ID 或描述"
							class="h-full min-w-0 flex-1 border-0 bg-transparent text-sm text-[var(--mp-text)] outline-none placeholder:text-[var(--mp-text-mute)]"
							@input="scheduleSearch"
							@keydown.enter.prevent="runSearchNow"
						/>
						<button v-if="keyword" type="button" aria-label="清除搜索" class="mp-search-clear" @click="clearSearch">
							<Icon name="x" size="xs" />
						</button>
					</div>
				</section>

				<!-- 工作区：左筛选面板 + 右结果区（展示带收进内容区顶部，不再整宽压场） -->
				<div class="mp-workspace">
					<!-- PC 左栏（<1024 隐藏，进底部 sheet） -->
					<aside class="mp-aside" aria-label="模型筛选">
						<FilterPanel
							:facets="facets"
							:category="category"
							:vendor="vendor"
							:capabilities="capabilities"
							:billing-mode="billingMode"
							@update:category="category = $event; onFilterChange()"
							@update:vendor="setVendor"
							@update:capabilities="capabilities = $event; onFilterChange()"
							@update:billing-mode="billingMode = $event; onFilterChange()"
							@reset="resetFilters"
						/>
					</aside>

					<div class="mp-content">
						<!-- 展示带：运营推荐位，收在内容区顶部（筛选激活时收起，把空间还给结果） -->
						<template v-if="showStrips">
							<ShowcaseStrip
								v-if="showcase.recommended.length"
								variant="recommended"
								:items="showcase.recommended"
								class="mb-4"
								@open="openDetail"
							/>
							<ShowcaseStrip
								v-if="showcase.newest.length"
								variant="newest"
								:items="showcase.newest"
								class="mb-4"
								@open="openDetail"
								@view-all="viewAllNewest"
							/>
						</template>

						<!-- 窄屏：品类 chips 横滑 + 筛选入口 -->
						<div class="mp-narrow-bar">
							<nav class="mp-cat-chips" aria-label="按模型类别筛选">
								<button
									v-for="cat in categoryMetaList"
									:key="cat.key"
									type="button"
									class="mp-cat-chip"
									:class="{ 'is-active': category === cat.value }"
									:aria-pressed="category === cat.value"
									@click="category = category === cat.value ? null : cat.value; onFilterChange()"
								>
									<Icon :name="cat.icon" size="xs" class="shrink-0" />
									{{ cat.shortLabel }}
								</button>
							</nav>
							<button type="button" class="mp-filter-btn" :class="{ 'is-active': hasActiveFilters }" @click="sheetVisible = true">
								<Icon name="cog" size="sm" />
								筛选
								<span v-if="activeChips.length" class="mp-filter-count">{{ activeChips.length }}</span>
							</button>
						</div>

						<!-- 工具栏：计数 + 已选 chips + 排序 -->
						<div class="mp-toolbar">
							<div class="flex min-w-0 flex-1 flex-wrap items-center gap-2">
								<p class="text-sm text-[var(--mp-text-dim)]">
									共 <strong class="font-mono font-semibold tabular-nums text-[var(--mp-text)]">{{ total }}</strong> 个模型
									<template v-if="keyword.trim()"> · “{{ keyword.trim() }}”</template>
								</p>
								<span v-for="chip in activeChips" :key="chip.key" class="mp-chip">
									{{ chip.label }}
									<button type="button" :aria-label="`移除筛选 ${chip.label}`" @click="chip.remove()">
										<Icon name="x" size="xs" />
									</button>
								</span>
								<button v-if="hasActiveFilters" type="button" class="mp-reset" @click="resetFilters">
									<Icon name="refresh" size="xs" />重置
								</button>
							</div>
							<label class="mp-sort">
								<span class="sr-only">排序方式</span>
								<Icon name="chart" size="xs" class="shrink-0 text-[var(--mp-text-mute)]" />
								<select v-model="sort" aria-label="排序方式" @change="onSortChange">
									<option v-for="opt in marketplaceSortOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
								</select>
							</label>
						</div>

						<!-- 目录 -->
						<section id="mp-catalog" class="scroll-mt-24">
							<!-- 加载骨架（与真实网格同 class，防加载态跳列） -->
							<div v-if="loading" class="mp-grid" aria-label="正在加载模型">
								<div v-for="i in 6" :key="i" class="mp-skeleton-card">
									<div class="mp-skeleton h-7 w-7 rounded-lg"></div>
									<div class="mp-skeleton mt-2 h-4 w-3/5 rounded"></div>
									<div class="mp-skeleton mt-1.5 h-3 w-2/5 rounded"></div>
									<div class="mp-skeleton mt-4 h-6 w-1/2 rounded"></div>
									<div class="mp-skeleton mt-3 h-3 w-full rounded"></div>
									<div class="mp-skeleton mt-1.5 h-3 w-4/5 rounded"></div>
									<div class="mp-skeleton mt-3 h-4 w-2/3 rounded"></div>
								</div>
							</div>

							<!-- 错误态 -->
							<div v-else-if="loadError" class="mp-empty">
								<Icon name="exclamationTriangle" size="xl" class="text-[var(--mp-text-mute)]" />
								<p class="text-base font-medium text-[var(--mp-text)]">模型列表加载失败</p>
								<p class="text-sm text-[var(--mp-text-dim)]">网络可能暂时不可用，请稍后重试。</p>
								<button type="button" class="mp-btn mp-btn-secondary mp-btn-sm mt-4" @click="loadModels">
									<Icon name="refresh" size="sm" />重新加载
								</button>
							</div>

							<!-- 空态 -->
							<div v-else-if="modelList.length === 0" class="mp-empty">
								<Icon name="search" size="xl" class="text-[var(--mp-text-mute)]" />
								<p class="text-base font-medium text-[var(--mp-text)]">没有找到匹配的模型</p>
								<p class="text-sm text-[var(--mp-text-dim)]">换一个关键词，或者清除当前筛选条件后再试。</p>
								<button type="button" class="mp-btn mp-btn-secondary mp-btn-sm mt-4" @click="resetFilters">
									<Icon name="refresh" size="sm" />重置筛选
								</button>
							</div>

							<!-- 卡片网格：key 携带筛选与页码，切换时整块淡入 -->
							<div v-else :key="`${category ?? 'all'}-${vendor ?? 'all'}-${capabilities.join(',')}-${billingMode}-${sort}-${currentPage}`" class="mp-grid mp-grid-in">
								<ModelCard v-for="model in modelList" :key="model.model_id" :model="model" @open="openDetail" @copy="id => copyText(id, '模型 ID 已复制')" />
							</div>

							<!-- 分页 -->
							<div v-if="!loading && !loadError && total > 0" class="mt-8">
								<BasePagination
									v-model="currentPage"
									v-model:page-size="pageSize"
									:total="total"
									:page-size-options="[24, 48]"
									@change="handlePageChange"
								/>
							</div>
						</section>
					</div>
				</div>
			</main>

			<footer class="relative border-t border-[var(--mp-line)] py-8">
				<div class="flex flex-col items-center gap-2 px-4 text-center">
					<div class="flex items-center gap-2">
						<img src="/favicon.png" :alt="siteName" class="h-5 w-5 rounded" />
						<span class="text-sm font-semibold text-[var(--mp-text-dim)]">{{ siteName }}</span>
					</div>
					<p class="text-xs text-[var(--mp-text-mute)]">统一的大模型 API 接入与管理平台</p>
					<span class="text-xs text-[var(--mp-text-mute)]">© {{ currentYear }} qianfree · AGPL-3.0</span>
				</div>
			</footer>

			<!-- 模型详情抽屉 -->
			<ModelDetailDrawer
				v-model:show="detailVisible"
				:model="selectedModel"
				:loading="detailLoading"
				:is-logged-in="isLoggedIn"
				:primary-action-route="primaryActionRoute"
				@copy="id => copyText(id, '模型 ID 已复制')"
				@copy-snippet="code => copyText(code, '代码已复制')"
			/>

			<!-- 窄屏筛选：底部 sheet（拇指热区 + 不与系统返回手势冲突） -->
			<n-drawer v-model:show="sheetVisible" placement="bottom" :height="'80dvh'" :auto-focus="false">
				<n-drawer-content title="筛选" closable>
					<div class="pb-4">
						<FilterPanel
							:facets="facets"
							:category="category"
							:vendor="vendor"
							:capabilities="capabilities"
							:billing-mode="billingMode"
							@update:category="category = $event; onFilterChange()"
							@update:vendor="setVendor"
							@update:capabilities="capabilities = $event; onFilterChange()"
							@update:billing-mode="billingMode = $event; onFilterChange()"
							@reset="resetFilters"
						/>
						<button type="button" class="mp-btn mp-btn-primary mp-btn-sm mt-6 w-full" @click="sheetVisible = false">
							查看 {{ total }} 个结果
						</button>
					</div>
				</n-drawer-content>
			</n-drawer>
		</div>
	</n-config-provider>
</template>

<style scoped>
/* ── 页面级 token（--mp-* 前缀，只作用于本页，不污染全局） ──
   默认明亮（与控制台浅色体系同族），[data-mp-theme='dark'] 覆盖为深色品牌线。
   浅底交互色统一用 teal-600（#0d9488 对白 4.5:1），深底用 teal-400。 */
.mp {
	--mp-bg: #f6f8fb;
	--mp-panel: #ffffff;
	--mp-panel-hover: #f7fafc;
	--mp-line: rgba(13, 32, 76, 0.1);
	--mp-text: #0d1626;
	--mp-text-dim: #4a5a76;
	--mp-text-mute: #8593a8;
	--mp-brand: #0d9488;
	--mp-brand-deep: #0f766e;
	--mp-accent: #0891b2;
	--mp-new: #4f46e5;
	--mp-deal: #e11d48;
	--mp-rec: #b45309;
	/* 派生 rgb 三元组（徽章底/描边/hover 光晕的 rgba 拼接） */
	--mp-brand-rgb: 13, 148, 136;
	--mp-accent-rgb: 8, 145, 178;
	--mp-new-rgb: 79, 70, 229;
	--mp-deal-rgb: 225, 29, 72;
	--mp-rec-rgb: 180, 83, 9;
	/* 品牌色底上的文字 */
	--mp-on-brand: #ffffff;
	/* 中性弱填充（chips/悬停底） */
	--mp-fill-faint: rgba(13, 32, 76, 0.04);
	/* 代码块底 */
	--mp-code-bg: #f0f4f8;
	/* 骨架屏渐变端点 */
	--mp-skeleton-a: rgba(13, 32, 76, 0.06);
	--mp-skeleton-b: rgba(13, 32, 76, 0.12);
	/* 装饰层 */
	--mp-grid-line: rgba(30, 70, 150, 0.045);
	--mp-glow-a: rgba(13, 148, 136, 0.08);
	--mp-glow-b: rgba(8, 145, 178, 0.06);
	/* 悬浮层（顶栏/侧栏/输入框） */
	--mp-header-bg: rgba(255, 255, 255, 0.85);
	--mp-aside-bg: rgba(255, 255, 255, 0.85);
	--mp-input-bg: #ffffff;
	--mp-font-mono: ui-monospace, 'SF Mono', 'JetBrains Mono', 'Cascadia Code', Consolas, 'Liberation Mono', monospace;

	position: relative;
	min-height: 100dvh;
	background: var(--mp-bg);
	color: var(--mp-text);
	color-scheme: light;
	overflow-x: clip;
	transition: background-color 0.25s ease, color 0.25s ease;
}

/* ── 暗色（用户切换） ── */
.mp[data-mp-theme='dark'] {
	--mp-bg: #060a14;
	--mp-panel: #0c1526;
	--mp-panel-hover: #111d33;
	--mp-line: rgba(148, 163, 199, 0.16);
	--mp-text: #e9eefb;
	--mp-text-dim: #93a0bf;
	--mp-text-mute: #5f6c8c;
	--mp-brand: #2dd4bf;
	--mp-brand-deep: #0d9488;
	--mp-accent: #22d3ee;
	--mp-new: #818cf8;
	--mp-deal: #fb7185;
	--mp-rec: #fbbf24;
	--mp-brand-rgb: 45, 212, 191;
	--mp-accent-rgb: 34, 211, 238;
	--mp-new-rgb: 129, 140, 248;
	--mp-deal-rgb: 251, 113, 133;
	--mp-rec-rgb: 251, 191, 36;
	--mp-on-brand: #032019;
	--mp-fill-faint: rgba(255, 255, 255, 0.03);
	--mp-code-bg: rgba(4, 8, 16, 0.6);
	--mp-skeleton-a: rgba(148, 163, 199, 0.08);
	--mp-skeleton-b: rgba(148, 163, 199, 0.16);
	--mp-grid-line: rgba(120, 165, 255, 0.035);
	--mp-glow-a: rgba(45, 212, 191, 0.07);
	--mp-glow-b: rgba(34, 211, 238, 0.05);
	--mp-header-bg: rgba(6, 10, 20, 0.78);
	--mp-aside-bg: rgba(12, 21, 38, 0.55);
	--mp-input-bg: rgba(12, 21, 38, 0.7);
	color-scheme: dark;
}

.mp-provider {
	display: block;
	min-height: 100dvh;
}

/* ── 装饰层 ── */
.mp-deco {
	position: absolute;
	inset: 0;
	z-index: 0;
	pointer-events: none;
	overflow: hidden;
}

.mp-deco-grid {
	position: absolute;
	inset: 0;
	background-image: linear-gradient(var(--mp-grid-line) 1px, transparent 1px),
		linear-gradient(90deg, var(--mp-grid-line) 1px, transparent 1px);
	background-size: 64px 64px;
	mask-image: linear-gradient(to bottom, rgba(0, 0, 0, 0.9), rgba(0, 0, 0, 0.25) 40%, transparent 70%);
}

.mp-deco-glow {
	position: absolute;
	width: 420px;
	height: 420px;
	border-radius: 50%;
	filter: blur(90px);
}

.mp-deco-glow-a {
	top: -160px;
	right: -100px;
	background: rgba(var(--mp-brand-rgb), 0.07);
}

.mp-deco-glow-b {
	top: 28%;
	left: -180px;
	background: var(--mp-glow-b);
}

/* ── 顶栏 ── */
.mp-header {
	position: sticky;
	top: 0;
	z-index: 40;
	background: var(--mp-header-bg);
	backdrop-filter: blur(16px);
	border-bottom: 1px solid var(--mp-line);
}

.mp-header-tag {
	margin-left: 4px;
	padding: 2px 10px;
	border: 1px solid rgba(var(--mp-brand-rgb), 0.35);
	border-radius: 999px;
	font-size: 11px;
	font-weight: 600;
	color: var(--mp-brand);
}

/* ── 页头带 ── */
.mp-stats {
	font-size: 13px;
	color: var(--mp-text-dim);
}

.mp-stats strong {
	color: var(--mp-text);
	font-weight: 700;
}

.mp-stats-dot {
	margin: 0 6px;
	color: var(--mp-text-mute);
}

.mp-search {
	display: flex;
	align-items: center;
	gap: 10px;
	height: 46px;
	margin-top: 14px;
	padding: 0 14px;
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	background: var(--mp-input-bg);
	transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.mp-search:focus-within {
	border-color: rgba(var(--mp-brand-rgb), 0.55);
	box-shadow: 0 0 0 3px rgba(var(--mp-brand-rgb), 0.12);
}

.mp-search input[type='search']::-webkit-search-cancel-button {
	display: none;
}

.mp-search-clear {
	display: grid;
	place-items: center;
	width: 24px;
	height: 24px;
	flex-shrink: 0;
	border: 0;
	border-radius: 50%;
	background: none;
	color: var(--mp-text-mute);
	cursor: pointer;
	transition: all 0.15s ease;
}

.mp-search-clear:hover {
	background: var(--mp-fill-faint);
	color: var(--mp-text);
}

/* ── 工作区 ── */
.mp-workspace {
	position: relative;
	z-index: 1;
	display: grid;
	grid-template-columns: 1fr;
	gap: 20px;
}

@media (min-width: 1024px) {
	.mp-workspace {
		grid-template-columns: 264px minmax(0, 1fr);
		align-items: start;
	}
}

.mp-aside {
	display: none;
}

@media (min-width: 1024px) {
	.mp-aside {
		display: block;
		position: sticky;
		top: 80px;
		max-height: calc(100dvh - 96px);
		overflow-y: auto;
		padding: 16px;
		border: 1px solid var(--mp-line);
		border-radius: 14px;
		background: var(--mp-aside-bg);
	}
}

.mp-content {
	min-width: 0;
}

/* ── 窄屏条：品类 chips + 筛选按钮 ── */
.mp-narrow-bar {
	display: flex;
	align-items: center;
	gap: 10px;
	margin-bottom: 12px;
}

@media (min-width: 1024px) {
	.mp-narrow-bar {
		display: none;
	}
}

.mp-cat-chips {
	display: flex;
	flex: 1;
	gap: 6px;
	overflow-x: auto;
	padding-bottom: 2px;
	scrollbar-width: none;
}

.mp-cat-chips::-webkit-scrollbar {
	display: none;
}

.mp-cat-chip {
	display: inline-flex;
	flex-shrink: 0;
	align-items: center;
	gap: 4px;
	padding: 5px 11px;
	border: 1px solid var(--mp-line);
	border-radius: 999px;
	background: none;
	color: var(--mp-text-dim);
	font-size: 12px;
	font-weight: 500;
	white-space: nowrap;
	cursor: pointer;
	transition: all 0.15s ease;
}

.mp-cat-chip:hover {
	color: var(--mp-text);
	border-color: rgba(148, 163, 199, 0.35);
}

.mp-cat-chip.is-active {
	background: rgba(var(--mp-brand-rgb), 0.12);
	border-color: rgba(var(--mp-brand-rgb), 0.5);
	color: var(--mp-brand);
}

.mp-filter-btn {
	display: inline-flex;
	flex-shrink: 0;
	align-items: center;
	gap: 5px;
	padding: 6px 12px;
	border: 1px solid var(--mp-line);
	border-radius: 999px;
	background: none;
	color: var(--mp-text-dim);
	font-size: 12px;
	cursor: pointer;
}

.mp-filter-btn.is-active {
	border-color: rgba(var(--mp-brand-rgb), 0.5);
	color: var(--mp-brand);
}

.mp-filter-count {
	display: grid;
	place-items: center;
	min-width: 16px;
	height: 16px;
	padding: 0 4px;
	border-radius: 999px;
	background: var(--mp-brand);
	color: var(--mp-on-brand);
	font-size: 10px;
	font-weight: 700;
}

/* ── 工具栏 ── */
.mp-toolbar {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: space-between;
	gap: 10px;
	margin-bottom: 14px;
}

.mp-chip {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	padding: 2px 6px 2px 10px;
	border: 1px solid rgba(var(--mp-brand-rgb), 0.35);
	border-radius: 999px;
	background: rgba(var(--mp-brand-rgb), 0.07);
	color: var(--mp-brand);
	font-size: 12px;
}

.mp-chip button {
	display: grid;
	place-items: center;
	width: 16px;
	height: 16px;
	border: 0;
	border-radius: 50%;
	background: none;
	color: inherit;
	cursor: pointer;
}

.mp-chip button:hover {
	background: rgba(var(--mp-brand-rgb), 0.2);
}

.mp-reset {
	display: inline-flex;
	align-items: center;
	gap: 4px;
	border: 0;
	background: none;
	color: var(--mp-text-dim);
	font-size: 12px;
	cursor: pointer;
}

.mp-reset:hover {
	color: var(--mp-text);
}

.mp-sort {
	display: inline-flex;
	align-items: center;
	gap: 6px;
	flex-shrink: 0;
}

.mp-sort select {
	border: 1px solid var(--mp-line);
	border-radius: 8px;
	background: var(--mp-panel);
	padding: 5px 8px;
	color: var(--mp-text-dim);
	font-size: 12px;
	outline: none;
	cursor: pointer;
}

.mp-sort select:focus-visible {
	border-color: rgba(var(--mp-brand-rgb), 0.55);
}

.mp-sort select option {
	background: var(--mp-panel);
	color: var(--mp-text);
}

/* ── 网格：动态列数（auto-fill 保末页不拉长条） ── */
.mp-grid {
	display: grid;
	gap: 16px;
	grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr));
}

.mp-grid-in {
	animation: mp-fade-in 0.25s ease-out;
}

@keyframes mp-fade-in {
	from {
		opacity: 0;
		transform: translateY(6px);
	}
	to {
		opacity: 1;
		transform: translateY(0);
	}
}

/* ── 骨架 / 空态 ── */
.mp-skeleton-card {
	padding: 18px;
	border: 1px solid var(--mp-line);
	border-radius: 12px;
	background: var(--mp-panel);
}

.mp-skeleton {
	background: linear-gradient(90deg, var(--mp-skeleton-a), var(--mp-skeleton-b), var(--mp-skeleton-a));
	background-size: 200% 100%;
	animation: mp-shimmer 1.6s ease-in-out infinite;
	border-radius: 6px;
}

@keyframes mp-shimmer {
	from {
		background-position: 200% 0;
	}
	to {
		background-position: -200% 0;
	}
}

.mp-empty {
	display: flex;
	flex-direction: column;
	align-items: center;
	gap: 6px;
	padding: 64px 16px;
	text-align: center;
	border: 1px dashed var(--mp-line);
	border-radius: 14px;
}

/* ── 按钮 ── */
.mp-btn {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: 6px;
	border-radius: 10px;
	font-weight: 600;
	cursor: pointer;
	transition: all 0.15s ease;
}

.mp-btn:focus-visible {
	outline: 2px solid var(--mp-brand);
	outline-offset: 2px;
}

.mp-btn-sm {
	padding: 7px 14px;
	font-size: 13px;
}

.mp-btn-primary {
	background: linear-gradient(135deg, var(--mp-brand), var(--mp-brand-deep));
	color: var(--mp-on-brand);
	box-shadow: 0 4px 20px rgba(var(--mp-brand-rgb), 0.25);
}

.mp-btn-primary:hover {
	filter: brightness(1.1);
}

.mp-btn-secondary {
	border: 1px solid rgba(148, 163, 199, 0.35);
	background: none;
	color: var(--mp-text-dim);
}

.mp-btn-secondary:hover {
	border-color: rgba(var(--mp-brand-rgb), 0.45);
	color: var(--mp-brand);
}

.mp-theme-toggle {
	display: grid;
	place-items: center;
	width: 32px;
	height: 32px;
	border: 1px solid var(--mp-line);
	border-radius: 8px;
	background: none;
	color: var(--mp-text-dim);
	cursor: pointer;
	transition: all 0.15s ease;
}

.mp-theme-toggle:hover {
	color: var(--mp-brand);
	border-color: rgba(var(--mp-brand-rgb), 0.45);
}

.mp-theme-toggle:focus-visible {
	outline: 2px solid var(--mp-brand);
	outline-offset: 2px;
}

.mp-btn-ghost {
	border: 0;
	background: none;
	color: var(--mp-text-dim);
}

.mp-btn-ghost:hover {
	color: var(--mp-text);
}

@media (prefers-reduced-motion: reduce) {
	.mp-grid-in {
		animation: none;
	}

	.mp-skeleton {
		animation: none;
	}
}
</style>
