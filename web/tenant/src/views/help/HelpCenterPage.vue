<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import MarkdownRenderer from '@/components/common/MarkdownRenderer.vue'
import request from '@/utils/request'

interface Category {
	id: number
	parent_id: number
	name: string
	slug: string
	description: string
	icon: string
	article_count: number
	children: Category[]
}

interface Article {
	id: number
	category_id: number
	title: string
	slug: string
	summary: string
	view_count: number
	published_at: string
}

interface ArticleDetail {
	id: number
	title: string
	slug: string
	content: string
	summary: string
	view_count: number
	keywords: string[]
	published_at: string
}

const categories = ref<Category[]>([])
const articles = ref<Article[]>([])
const loading = ref(false)
const articlesLoading = ref(false)
const articleLoading = ref(false)

const activeCategorySlug = ref<string | null>(null)
const activeArticleSlug = ref<string | null>(null)
const articleDetail = ref<ArticleDetail | null>(null)

const searchQuery = ref('')
const isSearchMode = ref(false)
const searchPage = ref(1)
const searchTotal = ref(0)
const pageSize = 20

const categoryColors = [
	{ bg: 'bg-primary-50', text: 'text-primary-600', dot: 'bg-primary-500' },
	{ bg: 'bg-blue-50', text: 'text-blue-600', dot: 'bg-blue-500' },
	{ bg: 'bg-purple-50', text: 'text-purple-600', dot: 'bg-purple-500' },
	{ bg: 'bg-amber-50', text: 'text-amber-600', dot: 'bg-amber-500' },
	{ bg: 'bg-emerald-50', text: 'text-emerald-600', dot: 'bg-emerald-500' },
	{ bg: 'bg-rose-50', text: 'text-rose-600', dot: 'bg-rose-500' },
	{ bg: 'bg-cyan-50', text: 'text-cyan-600', dot: 'bg-cyan-500' },
	{ bg: 'bg-indigo-50', text: 'text-indigo-600', dot: 'bg-indigo-500' },
]

const categoryIconList = ['bookOpen', 'document', 'cog', 'shield', 'key', 'creditCard', 'bell', 'chat', 'bolt', 'building']

function catColor(idx: number) {
	return categoryColors[idx % categoryColors.length]
}

function catIcon(idx: number): string {
	return categoryIconList[idx % categoryIconList.length]
}

const categoryIndexMap = computed(() => {
	const map = new Map<string, number>()
	categories.value.forEach((cat, idx) => {
		map.set(cat.slug, idx)
		cat.children?.forEach(child => map.set(child.slug, idx))
	})
	return map
})

const flattenedCategories = computed(() => {
	const result: Category[] = []
	function walk(items: Category[]) {
		for (const item of items) {
			result.push(item)
			if (item.children?.length) walk(item.children)
		}
	}
	walk(categories.value)
	return result
})

const activeCategoryName = computed(() => {
	if (!activeCategorySlug.value) return ''
	const cat = flattenedCategories.value.find(c => c.slug === activeCategorySlug.value)
	return cat?.name || ''
})

const activeColorIndex = computed(() => {
	if (!activeCategorySlug.value) return 0
	return categoryIndexMap.value.get(activeCategorySlug.value) ?? 0
})

const isLanding = computed(() =>
	!activeCategorySlug.value && !isSearchMode.value && !articleDetail.value && !articlesLoading.value
)

async function fetchCategories() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/help/categories')
		const raw = res.data?.data
		categories.value = Array.isArray(raw) ? raw : (raw?.list || [])
	} catch {
		categories.value = []
	} finally {
		loading.value = false
	}
}

async function fetchArticles(slug: string) {
	articlesLoading.value = true
	activeCategorySlug.value = slug
	activeArticleSlug.value = null
	articleDetail.value = null
	isSearchMode.value = false
	try {
		const res: any = await request.get(`/tenant/help/categories/${slug}/articles`, {
			params: { page: 1, page_size: pageSize },
		})
		const raw = res.data?.data
		articles.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
	} catch {
		articles.value = []
	} finally {
		articlesLoading.value = false
	}
}

async function fetchArticleDetail(slug: string) {
	articleLoading.value = true
	activeArticleSlug.value = slug
	try {
		const res: any = await request.get(`/tenant/help/articles/${slug}`)
		articleDetail.value = res.data?.data || null
	} catch {
		articleDetail.value = null
	} finally {
		articleLoading.value = false
	}
}

async function handleSearch() {
	const q = searchQuery.value.trim()
	if (!q) return
	isSearchMode.value = true
	activeCategorySlug.value = null
	activeArticleSlug.value = null
	articleDetail.value = null
	articlesLoading.value = true
	searchPage.value = 1
	try {
		const res: any = await request.get('/tenant/help/search', {
			params: { q, page: 1, page_size: pageSize },
		})
		const raw = res.data?.data
		articles.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		searchTotal.value = raw?.total || 0
	} catch {
		articles.value = []
		searchTotal.value = 0
	} finally {
		articlesLoading.value = false
	}
}

function goBackToList() {
	activeArticleSlug.value = null
	articleDetail.value = null
}

function goBackToLanding() {
	activeCategorySlug.value = null
	activeArticleSlug.value = null
	articleDetail.value = null
	articles.value = []
	isSearchMode.value = false
}

onMounted(fetchCategories)
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="flex items-center justify-between gap-4">
			<div>
				<h1 class="page-title">帮助中心</h1>
				<p class="page-description">浏览帮助文档，快速找到常见问题的解答</p>
			</div>
			<div class="relative flex-shrink-0">
				<Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
				<input
					v-model="searchQuery"
					class="h-9 w-44 sm:w-52 lg:w-64 rounded-lg bg-gray-100 border border-transparent pl-9 pr-8 text-sm text-gray-700 placeholder:text-gray-400 focus:bg-white focus:border-primary-300 focus:outline-none focus:ring-2 focus:ring-primary-500/20 transition-all"
					placeholder="搜索帮助文章..."
					@keydown.enter="handleSearch"
				/>
				<button
					v-if="searchQuery"
					class="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 transition-colors"
					@click="searchQuery = ''"
				>
					<Icon name="x" size="xs" />
				</button>
			</div>
		</div>

		<!-- Mobile: horizontal category pills -->
		<div v-if="categories.length" class="lg:hidden flex gap-2 overflow-x-auto scrollbar-hide -mx-1 px-1">
			<button
				:class="[
					'flex-shrink-0 px-3 py-1.5 rounded-full text-xs font-medium transition-colors',
					!activeCategorySlug && !isSearchMode
						? 'bg-primary-500 text-white'
						: 'bg-gray-100 text-gray-600 hover:bg-gray-200'
				]"
				@click="goBackToLanding"
			>
				全部
			</button>
			<button
				v-for="(cat, idx) in categories"
				:key="cat.id"
				:class="[
					'flex-shrink-0 px-3 py-1.5 rounded-full text-xs font-medium transition-colors flex items-center gap-1.5',
					activeCategorySlug === cat.slug && !isSearchMode
						? 'bg-primary-500 text-white'
						: 'bg-gray-100 text-gray-600 hover:bg-gray-200'
				]"
				@click="fetchArticles(cat.slug)"
			>
				<span class="w-1.5 h-1.5 rounded-full" :class="catColor(idx).dot" />
				{{ cat.name }}
			</button>
		</div>

		<!-- Main Layout -->
		<div class="grid grid-cols-1 lg:grid-cols-4 gap-6">
			<!-- Desktop Sidebar -->
			<div class="hidden lg:block lg:col-span-1">
				<div class="card sticky top-6">
					<div class="px-4 py-3 border-b border-gray-100">
						<h3 class="text-sm font-semibold text-gray-700">帮助分类</h3>
					</div>
					<nav class="p-2 space-y-0.5">
						<button
							:class="[
								'w-full text-left px-3 py-2 rounded-lg text-sm transition-colors flex items-center gap-2',
								!activeCategorySlug && !isSearchMode
									? 'bg-primary-50 text-primary-700 font-medium'
									: 'text-gray-600 hover:bg-gray-50'
							]"
							@click="goBackToLanding"
						>
							<Icon name="grid" size="sm" />
							<span>全部</span>
						</button>

						<template v-for="(cat, idx) in categories" :key="cat.id">
							<button
								:class="[
									'w-full text-left px-3 py-2 rounded-lg text-sm transition-colors flex items-center gap-2.5',
									activeCategorySlug === cat.slug && !isSearchMode
										? 'bg-gray-50 font-medium text-gray-900'
										: 'text-gray-600 hover:bg-gray-50'
								]"
								@click="fetchArticles(cat.slug)"
							>
								<span class="w-2 h-2 rounded-full flex-shrink-0" :class="catColor(idx).dot" />
								<span class="flex-1 truncate">{{ cat.name }}</span>
								<span class="text-xs text-gray-400 tabular-nums">{{ cat.article_count }}</span>
							</button>
							<div v-if="cat.children?.length" class="ml-4 pl-3 border-l border-gray-100 space-y-0.5 py-0.5">
								<button
									v-for="child in cat.children"
									:key="child.id"
									:class="[
										'w-full text-left px-3 py-1.5 rounded-lg text-xs transition-colors flex items-center justify-between',
										activeCategorySlug === child.slug && !isSearchMode
											? 'bg-gray-50 font-medium text-gray-900'
											: 'text-gray-500 hover:bg-gray-50'
									]"
									@click="fetchArticles(child.slug)"
								>
									<span class="truncate">{{ child.name }}</span>
									<span class="text-xs text-gray-400 tabular-nums ml-2">{{ child.article_count }}</span>
								</button>
							</div>
						</template>
					</nav>
					<div v-if="!categories.length && !loading" class="px-4 py-6 text-center">
						<p class="text-xs text-gray-400">暂无分类</p>
					</div>
				</div>
			</div>

			<!-- Content Area -->
			<div class="lg:col-span-3">
				<!-- Loading -->
				<div v-if="articlesLoading || articleLoading" class="card p-12 text-center">
					<div class="spinner mx-auto mb-3" />
					<p class="text-sm text-gray-500">加载中...</p>
				</div>

				<!-- Article Detail View -->
				<template v-else-if="articleDetail">
					<div class="card">
						<div class="px-6 py-3 border-b border-gray-100 flex items-center gap-2 text-sm">
							<button
								class="text-primary-600 hover:text-primary-700 transition-colors flex items-center gap-1"
								@click="goBackToList"
							>
								<Icon name="arrowLeft" size="xs" />
								{{ isSearchMode ? '搜索结果' : activeCategoryName }}
							</button>
							<Icon name="chevronRight" size="xs" class="text-gray-300" />
							<span class="text-gray-500 truncate">{{ articleDetail.title }}</span>
						</div>
						<div class="px-6 py-6">
							<h1 class="text-xl font-bold text-gray-900 mb-3">{{ articleDetail.title }}</h1>
							<div class="flex items-center gap-4 text-xs text-gray-400 mb-6 pb-6 border-b border-gray-100">
								<span class="flex items-center gap-1">
									<Icon name="eye" size="xs" />
									{{ articleDetail.view_count }} 次浏览
								</span>
								<span v-if="articleDetail.published_at" class="flex items-center gap-1">
									<Icon name="calendar" size="xs" />
									{{ articleDetail.published_at }}
								</span>
							</div>
							<div v-if="articleDetail.keywords?.length" class="flex flex-wrap gap-1.5 mb-6">
								<span v-for="kw in articleDetail.keywords" :key="kw" class="badge badge-gray">
									{{ kw }}
								</span>
							</div>
							<MarkdownRenderer :content="articleDetail.content" />
						</div>
					</div>
				</template>

				<!-- Landing: Category Cards -->
				<template v-else-if="isLanding">
					<div v-if="categories.length > 0" class="grid grid-cols-1 sm:grid-cols-2 gap-4">
						<div
							v-for="(cat, idx) in categories"
							:key="cat.id"
							class="card card-hover cursor-pointer group p-5 flex items-start gap-4"
							@click="fetchArticles(cat.slug)"
						>
							<div
								class="h-10 w-10 rounded-xl flex items-center justify-center flex-shrink-0"
								:class="catColor(idx).bg"
							>
								<Icon :name="cat.icon || catIcon(idx)" size="md" :class="catColor(idx).text" />
							</div>
							<div class="flex-1 min-w-0">
								<h3 class="text-sm font-semibold text-gray-900 group-hover:text-primary-600 transition-colors">
									{{ cat.name }}
								</h3>
								<p v-if="cat.description" class="text-xs text-gray-500 mt-1 line-clamp-2">{{ cat.description }}</p>
								<div class="flex items-center gap-1 mt-2 text-xs text-gray-400">
									<Icon name="document" size="xs" />
									{{ cat.article_count }} 篇文章
								</div>
							</div>
						</div>
					</div>
					<div v-else class="card p-12 text-center">
						<Icon name="bookOpen" size="xl" class="mx-auto mb-4 text-gray-300" />
						<p class="text-gray-500">暂无帮助文档</p>
					</div>
				</template>

				<!-- Article List View -->
				<template v-else-if="articles.length > 0">
					<div v-if="isSearchMode" class="flex items-center gap-2 text-sm text-gray-500 mb-4">
						<Icon name="search" size="sm" class="text-gray-400" />
						搜索 "{{ searchQuery }}" 找到 <span class="font-medium text-gray-700">{{ searchTotal }}</span> 条结果
					</div>
					<div v-else-if="activeCategorySlug" class="flex items-center gap-2 text-sm text-gray-500 mb-4">
						<span class="w-2 h-2 rounded-full" :class="catColor(activeColorIndex).dot" />
						<span class="font-medium text-gray-700">{{ activeCategoryName }}</span>
					</div>
					<div class="space-y-2">
						<div
							v-for="article in articles"
							:key="article.id"
							class="card card-hover cursor-pointer group"
							@click="fetchArticleDetail(article.slug)"
						>
							<div class="px-5 py-4">
								<div class="flex items-start justify-between gap-3">
									<div class="flex-1 min-w-0">
										<h3 class="text-sm font-medium text-gray-900 group-hover:text-primary-600 transition-colors">
											{{ article.title }}
										</h3>
										<p v-if="article.summary" class="text-xs text-gray-500 mt-1.5 line-clamp-2">
											{{ article.summary }}
										</p>
									</div>
									<Icon name="chevronRight" size="sm" class="text-gray-300 group-hover:text-primary-400 transition-colors mt-0.5 flex-shrink-0" />
								</div>
								<div class="flex items-center gap-3 mt-2.5 text-xs text-gray-400">
									<span class="flex items-center gap-1">
										<Icon name="eye" size="xs" />
										{{ article.view_count }}
									</span>
									<span v-if="article.published_at">{{ article.published_at }}</span>
								</div>
							</div>
						</div>
					</div>
				</template>

				<!-- Empty search results -->
				<div v-else-if="isSearchMode" class="card p-12 text-center">
					<Icon name="search" size="xl" class="mx-auto mb-4 text-gray-300" />
					<p class="text-gray-500">未找到相关文章，请尝试其他关键词</p>
				</div>

				<!-- Empty category -->
				<div v-else class="card p-12 text-center">
					<Icon name="document" size="xl" class="mx-auto mb-4 text-gray-300" />
					<p class="text-gray-500">该分类下暂无文章</p>
				</div>
			</div>
		</div>
	</div>
</template>
