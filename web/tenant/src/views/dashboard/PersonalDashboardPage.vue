<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import TokenTrendChart from '@/components/charts/TokenTrendChart.vue'
import ModelDistChart from '@/components/charts/ModelDistChart.vue'
import request from '@/utils/request'

// ===== State =====
const loading = ref(false)
const chartsLoading = ref(false)
const selectedDays = ref(7)

// ===== Types =====
interface DayStats { requests: number; input_tokens: number; output_tokens: number; total_cost: number }
interface ErrorRate { total: number; success: number; error: number; timeout: number; cancelled: number; rate: number }
interface Latency { avg_ms: number; p50_ms: number; p95_ms: number; p99_ms: number; avg_first_token_ms: number }
interface Cache { cache_creation_tokens: number; cache_read_tokens: number; total_input_tokens: number; hit_ratio: number }
interface ReqTypeItem { type: string; label: string; requests: number; percentage: number }
interface QuotaStatus { quota_type: string; quota_limit: number; quota_used: number; period: string; usage_percent: number; next_reset_at?: string }
interface OverviewData {
	today: DayStats
	month: DayStats
	error_rate: ErrorRate
	latency: Latency
	cache: Cache
	request_types: ReqTypeItem[]
	quota?: QuotaStatus
}
interface TrendPoint { date: string; input_tokens: number; output_tokens: number; requests: number; total_cost: number }
interface ModelItem { model_name: string; requests: number; input_tokens: number; output_tokens: number; total_cost: number }
interface ApiKeyItem { api_key_id: number; key_name: string; key_prefix: string; requests: number; input_tokens: number; output_tokens: number; total_cost: number }

// ===== Data =====
const overviewData = ref<OverviewData | null>(null)
const trendData = ref<TrendPoint[]>([])
const modelData = ref<ModelItem[]>([])
const apiKeyData = ref<ApiKeyItem[]>([])

// ===== Helpers =====
function formatNumber(n: any): string {
	const v = Number(n) || 0
	if (v >= 1_000_000_000) return (v / 1_000_000_000).toFixed(1) + 'B'
	if (v >= 1_000_000) return (v / 1_000_000).toFixed(1) + 'M'
	if (v >= 1_000) return (v / 1_000).toFixed(1) + 'K'
	return String(v)
}

function formatCost(n: any): string {
	const v = Number(n) || 0
	if (v >= 1) return '$' + v.toFixed(2)
	if (v >= 0.01) return '$' + v.toFixed(4)
	return '$' + v.toFixed(6)
}

function formatMs(n: any): string {
	const v = Number(n) || 0
	if (v >= 1000) return (v / 1000).toFixed(1) + 's'
	return Math.round(v) + 'ms'
}

function ensureArray<T>(val: any): T[] {
	if (Array.isArray(val)) return val
	return []
}

// ===== Core Stats =====
const coreStats = computed(() => {
	if (!overviewData.value) return []
	const today = overviewData.value.today
	const month = overviewData.value.month
	const totalMonthTokens = month.input_tokens + month.output_tokens
	return [
		{
			label: '今日请求',
			value: formatNumber(today.requests),
			sub: `输入 ${formatNumber(today.input_tokens)} / 输出 ${formatNumber(today.output_tokens)}`,
			icon: 'play',
			iconClass: 'bg-blue-100 text-blue-600',
		},
		{
			label: '本月请求',
			value: formatNumber(month.requests),
			sub: `日均 ${formatNumber(month.requests / (new Date().getDate() || 1))}`,
			icon: 'chart',
			iconClass: 'bg-emerald-100 text-emerald-600',
		},
		{
			label: '本月 Token',
			value: formatNumber(totalMonthTokens),
			sub: `输入 ${formatNumber(month.input_tokens)} / 输出 ${formatNumber(month.output_tokens)}`,
			icon: 'bolt',
			iconClass: 'bg-amber-100 text-amber-600',
		},
		{
			label: '本月消费',
			value: formatCost(month.total_cost),
			sub: `今日 ${formatCost(today.total_cost)}`,
			icon: 'creditCard',
			iconClass: 'bg-rose-100 text-rose-600',
		},
	]
})

// ===== Data Loading =====
async function fetchOverview() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/personal-dashboard')
		overviewData.value = res.data?.data || null
	} catch {
		overviewData.value = null
	} finally {
		loading.value = false
	}
}

async function fetchCharts() {
	chartsLoading.value = true
	try {
		const [trendRes, modelRes, keyRes]: any[] = await Promise.all([
			request.get('/tenant/personal-dashboard/trends', { params: { days: selectedDays.value } }),
			request.get('/tenant/personal-dashboard/models', { params: { days: selectedDays.value } }),
			request.get('/tenant/personal-dashboard/api-key-usage', { params: { days: selectedDays.value } }),
		])
		trendData.value = ensureArray(trendRes.data?.data?.list)
		modelData.value = ensureArray(modelRes.data?.data?.list)
		apiKeyData.value = ensureArray(keyRes.data?.data?.list)
	} catch {
		trendData.value = []
		modelData.value = []
		apiKeyData.value = []
	} finally {
		chartsLoading.value = false
	}
}

function refreshAll() {
	fetchOverview()
	fetchCharts()
}

watch(selectedDays, () => {
	fetchCharts()
})

onMounted(() => {
	refreshAll()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<div class="flex items-center justify-between">
				<div>
					<h1 class="page-title">个人看板</h1>
					<p class="page-description">我的 AI 使用统计和消费分析</p>
				</div>
				<button class="btn btn-secondary btn-sm" @click="refreshAll">
					<Icon name="refresh" size="sm" />
					刷新
				</button>
			</div>
		</div>

		<!-- ===== Row 1: Core Stats ===== -->
		<div v-if="loading" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
			<div v-for="i in 4" :key="i" class="stat-card">
				<div class="skeleton h-12 w-12 rounded-xl"></div>
				<div class="flex-1">
					<div class="skeleton h-4 w-16 mb-2"></div>
					<div class="skeleton h-7 w-24"></div>
				</div>
			</div>
		</div>
		<div v-else-if="overviewData" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
			<div v-for="stat in coreStats" :key="stat.label" class="stat-card">
				<div class="stat-icon" :class="stat.iconClass">
					<Icon :name="stat.icon" size="lg" />
				</div>
				<div class="min-w-0 flex-1">
					<p class="stat-label">{{ stat.label }}</p>
					<p class="stat-value">{{ stat.value }}</p>
					<p class="text-xs text-gray-400 mt-0.5 truncate">{{ stat.sub }}</p>
				</div>
			</div>
		</div>

		<!-- ===== Row 2: Quota Status ===== -->
		<div v-if="overviewData?.quota" class="card p-5">
			<div class="flex items-center justify-between mb-3">
				<div class="flex items-center gap-2">
					<Icon name="shield" size="md" class="text-primary-500" />
					<span class="text-sm font-medium text-gray-900">额度使用</span>
					<span class="badge badge-gray">{{ overviewData.quota.quota_type === 'periodic' ? overviewData.quota.period : overviewData.quota.quota_type }}</span>
				</div>
				<span class="text-sm font-semibold" :class="overviewData.quota.usage_percent >= 90 ? 'text-red-600' : overviewData.quota.usage_percent >= 70 ? 'text-amber-600' : 'text-emerald-600'">
					{{ overviewData.quota.usage_percent.toFixed(1) }}%
				</span>
			</div>
			<div class="progress">
				<div
					class="progress-bar"
					:style="{ width: Math.min(overviewData.quota.usage_percent, 100) + '%' }"
					:class="overviewData.quota.usage_percent >= 90 ? '!bg-gradient-to-r !from-red-500 !to-red-400' : overviewData.quota.usage_percent >= 70 ? '!bg-gradient-to-r !from-amber-500 !to-amber-400' : ''"
				></div>
			</div>
			<div class="flex justify-between mt-2">
				<span class="text-xs text-gray-500">已用 ${{ overviewData.quota.quota_used?.toFixed(2) || '0.00' }}</span>
				<span class="text-xs text-gray-500">额度 ${{ overviewData.quota.quota_limit?.toFixed(2) || '0.00' }}</span>
			</div>
		</div>

		<!-- ===== Row 3: Charts Header + Day Selector ===== -->
		<div class="flex items-center justify-between">
			<h2 class="text-base font-semibold text-gray-900">数据趋势</h2>
			<div class="flex items-center gap-1 p-1 rounded-xl bg-gray-100">
				<button
					v-for="d in [7, 30, 90]"
					:key="d"
					class="tab"
					:class="{ 'tab-active': selectedDays === d }"
					@click="selectedDays = d"
				>
					{{ d }} 天
				</button>
			</div>
		</div>

		<!-- ===== Row 4: Charts ===== -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<TokenTrendChart :data="trendData" :loading="chartsLoading" />
			<ModelDistChart :data="modelData" :loading="chartsLoading" />
		</div>

		<!-- ===== Row 5: Stats Grid (Request Types + Error + Latency + Cache) ===== -->
		<div v-if="overviewData" class="grid grid-cols-1 md:grid-cols-3 gap-4">
			<!-- Request Type Distribution -->
			<div class="card p-5">
				<h3 class="text-sm font-semibold text-gray-900 mb-4">请求类型分布</h3>
				<div v-if="overviewData.request_types?.length" class="space-y-3">
					<div v-for="rt in overviewData.request_types" :key="rt.type">
						<div class="flex items-center justify-between mb-1">
							<span class="text-sm text-gray-700">{{ rt.label }}</span>
							<span class="text-xs text-gray-500">{{ rt.requests }} 次 ({{ rt.percentage.toFixed(1) }}%)</span>
						</div>
						<div class="h-2 rounded-full bg-gray-100 overflow-hidden">
							<div
								class="h-full rounded-full bg-gradient-to-r from-primary-500 to-primary-400 transition-all duration-500"
								:style="{ width: rt.percentage + '%' }"
							></div>
						</div>
					</div>
				</div>
				<div v-else class="text-center text-gray-400 text-sm py-4">暂无数据</div>
			</div>

			<!-- Error Rate + Latency -->
			<div class="card p-5">
				<h3 class="text-sm font-semibold text-gray-900 mb-4">质量指标</h3>
				<div class="space-y-4">
					<!-- Success Rate -->
					<div>
						<div class="flex items-center justify-between mb-1">
							<span class="text-sm text-gray-700">成功率</span>
							<span class="text-sm font-semibold" :class="overviewData.error_rate.rate >= 0.99 ? 'text-emerald-600' : overviewData.error_rate.rate >= 0.95 ? 'text-amber-600' : 'text-red-600'">
								{{ (overviewData.error_rate.rate * 100).toFixed(1) }}%
							</span>
						</div>
						<div class="flex gap-2 text-xs text-gray-400">
							<span>成功 {{ overviewData.error_rate.success }}</span>
							<span>错误 {{ overviewData.error_rate.error }}</span>
							<span>超时 {{ overviewData.error_rate.timeout }}</span>
						</div>
					</div>
					<!-- Latency -->
					<div>
						<span class="text-sm text-gray-700">响应延迟</span>
						<div class="grid grid-cols-2 gap-2 mt-2">
							<div class="bg-gray-50 rounded-lg px-3 py-2">
								<p class="text-xs text-gray-400">平均</p>
								<p class="text-sm font-semibold text-gray-900">{{ formatMs(overviewData.latency.avg_ms) }}</p>
							</div>
							<div class="bg-gray-50 rounded-lg px-3 py-2">
								<p class="text-xs text-gray-400">首 Token</p>
								<p class="text-sm font-semibold text-gray-900">{{ formatMs(overviewData.latency.avg_first_token_ms) }}</p>
							</div>
							<div class="bg-gray-50 rounded-lg px-3 py-2">
								<p class="text-xs text-gray-400">P95</p>
								<p class="text-sm font-semibold text-gray-900">{{ formatMs(overviewData.latency.p95_ms) }}</p>
							</div>
							<div class="bg-gray-50 rounded-lg px-3 py-2">
								<p class="text-xs text-gray-400">P99</p>
								<p class="text-sm font-semibold text-gray-900">{{ formatMs(overviewData.latency.p99_ms) }}</p>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Cache Utilization -->
			<div class="card p-5">
				<h3 class="text-sm font-semibold text-gray-900 mb-4">缓存利用</h3>
				<div class="space-y-4">
					<div class="text-center">
						<p class="text-3xl font-bold" :class="overviewData.cache.hit_ratio >= 0.5 ? 'text-emerald-600' : overviewData.cache.hit_ratio > 0 ? 'text-amber-600' : 'text-gray-400'">
							{{ (overviewData.cache.hit_ratio * 100).toFixed(1) }}%
						</p>
						<p class="text-xs text-gray-400 mt-1">缓存命中率</p>
					</div>
					<div class="space-y-2">
						<div class="flex items-center justify-between">
							<span class="text-sm text-gray-500">缓存读取</span>
							<span class="text-sm font-medium text-gray-900">{{ formatNumber(overviewData.cache.cache_read_tokens) }}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-sm text-gray-500">缓存写入</span>
							<span class="text-sm font-medium text-gray-900">{{ formatNumber(overviewData.cache.cache_creation_tokens) }}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-sm text-gray-500">输入 Token</span>
							<span class="text-sm font-medium text-gray-900">{{ formatNumber(overviewData.cache.total_input_tokens) }}</span>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- ===== Row 6: Tables ===== -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- API Key Usage -->
			<div class="card p-6">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-base font-semibold text-gray-900">API Key 用量</h3>
					<span class="text-xs text-gray-400">近 {{ selectedDays }} 天</span>
				</div>

				<div v-if="chartsLoading" class="space-y-3">
					<div v-for="i in 3" :key="i" class="skeleton h-10 rounded-lg"></div>
				</div>

				<div v-else-if="apiKeyData.length === 0" class="py-8 text-center text-gray-400 text-sm">
					暂无 API Key 使用数据
				</div>

				<div v-else class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>密钥</th>
								<th class="text-right">请求</th>
								<th class="text-right">Token</th>
								<th class="text-right">费用</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="key in apiKeyData" :key="key.api_key_id">
								<td>
									<div>
										<p class="font-medium text-gray-900 truncate max-w-[120px]">{{ key.key_name }}</p>
										<p class="text-xs text-gray-400 font-mono">{{ key.key_prefix }}***</p>
									</div>
								</td>
								<td class="text-right text-sm tabular-nums">{{ formatNumber(key.requests) }}</td>
								<td class="text-right text-sm tabular-nums">{{ formatNumber(key.input_tokens + key.output_tokens) }}</td>
								<td class="text-right text-sm font-medium text-emerald-600 tabular-nums">{{ formatCost(key.total_cost) }}</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>

			<!-- Top Models Table -->
			<div class="card p-6">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-base font-semibold text-gray-900">模型用量详情</h3>
					<span class="text-xs text-gray-400">近 {{ selectedDays }} 天</span>
				</div>

				<div v-if="chartsLoading" class="space-y-3">
					<div v-for="i in 3" :key="i" class="skeleton h-10 rounded-lg"></div>
				</div>

				<div v-else-if="modelData.length === 0" class="py-8 text-center text-gray-400 text-sm">
					暂无模型使用数据
				</div>

				<div v-else class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>模型</th>
								<th class="text-right">请求</th>
								<th class="text-right">输入</th>
								<th class="text-right">输出</th>
								<th class="text-right">费用</th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="model in modelData" :key="model.model_name">
								<td class="font-medium text-gray-900 text-sm truncate max-w-[160px]">{{ model.model_name }}</td>
								<td class="text-right text-sm tabular-nums">{{ formatNumber(model.requests) }}</td>
								<td class="text-right text-sm tabular-nums">{{ formatNumber(model.input_tokens) }}</td>
								<td class="text-right text-sm tabular-nums">{{ formatNumber(model.output_tokens) }}</td>
								<td class="text-right text-sm font-medium text-emerald-600 tabular-nums">{{ formatCost(model.total_cost) }}</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>
		</div>

		<!-- ===== Empty State ===== -->
		<div v-if="!loading && !overviewData" class="card">
			<div class="empty-state">
				<Icon name="chart" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无数据</p>
				<p class="empty-state-description">您的 AI 使用数据将在首次调用后展示</p>
			</div>
		</div>
	</div>
</template>
