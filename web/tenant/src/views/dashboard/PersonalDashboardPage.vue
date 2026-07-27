<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import ModelDistChart from '@/components/charts/ModelDistChart.vue'
import TokenTrendChart from '@/components/charts/TokenTrendChart.vue'
import { useTenantAuthStore } from '@/stores/tenant-auth'
import request from '@/utils/request'

interface DayStats {
	requests: number
	input_tokens: number
	output_tokens: number
	total_cost: number
}

interface ErrorRate {
	total: number
	success: number
	error: number
	timeout: number
	cancelled: number
	rate: number
}

interface Latency {
	avg_ms: number
	p50_ms: number
	p95_ms: number
	p99_ms: number
	avg_first_token_ms: number
}

interface Cache {
	cache_creation_tokens: number
	cache_read_tokens: number
	total_input_tokens: number
	hit_ratio: number
}

interface ReqTypeItem {
	type: string
	label: string
	requests: number
	percentage: number
}

interface QuotaStatus {
	quota_type: string
	quota_limit: number
	quota_used: number
	period: string
	usage_percent: number
	next_reset_at?: string
}

interface OverviewData {
	today: DayStats
	month: DayStats
	error_rate: ErrorRate
	latency: Latency
	cache: Cache
	request_types: ReqTypeItem[]
	quota?: QuotaStatus
}

interface TrendPoint {
	date: string
	input_tokens: number
	output_tokens: number
	requests: number
	total_cost: number
}

interface ModelItem {
	model_name: string
	requests: number
	input_tokens: number
	output_tokens: number
	total_cost: number
}

interface ApiKeyItem {
	api_key_id: number
	key_name: string
	key_prefix: string
	requests: number
	input_tokens: number
	output_tokens: number
	total_cost: number
}

const authStore = useTenantAuthStore()
const loading = ref(false)
const chartsLoading = ref(false)
const selectedDays = ref(7)
const overviewData = ref<OverviewData | null>(null)
const trendData = ref<TrendPoint[]>([])
const modelData = ref<ModelItem[]>([])
const apiKeyData = ref<ApiKeyItem[]>([])

function formatNumber(value: unknown): string {
	const number = Number(value) || 0
	if (number >= 1_000_000_000) return `${(number / 1_000_000_000).toFixed(1)}B`
	if (number >= 1_000_000) return `${(number / 1_000_000).toFixed(1)}M`
	if (number >= 1_000) return `${(number / 1_000).toFixed(1)}K`
	return number.toLocaleString('zh-CN')
}

function formatCost(value: unknown): string {
	const number = Number(value) || 0
	if (number === 0) return '$0.00'
	if (number >= 1) return `$${number.toFixed(2)}`
	if (number >= 0.01) return `$${number.toFixed(4)}`
	return `$${number.toFixed(6)}`
}

function formatMs(value: unknown): string {
	const number = Number(value) || 0
	if (number >= 1000) return `${(number / 1000).toFixed(1)}s`
	return `${Math.round(number)}ms`
}

function formatResetAt(value?: string): string {
	if (!value) return '按额度周期重置'
	const date = new Date(value.replace(' ', 'T'))
	if (Number.isNaN(date.getTime())) return value
	return `${date.getMonth() + 1} 月 ${date.getDate()} 日重置`
}

function ensureArray<T>(value: unknown): T[] {
	return Array.isArray(value) ? value : []
}

const greeting = computed(() => {
	const hour = new Date().getHours()
	if (hour < 6) return '夜深了'
	if (hour < 12) return '早上好'
	if (hour < 18) return '下午好'
	return '晚上好'
})

const formattedDate = computed(() =>
	new Intl.DateTimeFormat('zh-CN', {
		month: 'long',
		day: 'numeric',
		weekday: 'long',
	}).format(new Date()),
)

const periodSummary = computed(() =>
	trendData.value.reduce(
		(summary, point) => ({
			requests: summary.requests + Number(point.requests || 0),
			tokens: summary.tokens + Number(point.input_tokens || 0) + Number(point.output_tokens || 0),
			cost: summary.cost + Number(point.total_cost || 0),
		}),
		{ requests: 0, tokens: 0, cost: 0 },
	),
)

const coreStats = computed(() => {
	if (!overviewData.value) return []
	const { today, error_rate: errorRate, latency } = overviewData.value
	const failedRequests = errorRate.error + errorRate.timeout + errorRate.cancelled
	return [
		{
			label: '今日请求',
			value: formatNumber(today.requests),
			description: `${formatNumber(today.input_tokens + today.output_tokens)} Token`,
			badge: '今日',
			icon: 'play',
			color: '#7667f6',
			soft: 'rgba(118, 103, 246, 0.14)',
		},
		{
			label: `近 ${selectedDays.value} 天消费`,
			value: formatCost(periodSummary.value.cost),
			description: `${formatNumber(periodSummary.value.requests)} 次调用`,
			badge: '费用',
			icon: 'wallet',
			color: '#3b9df8',
			soft: 'rgba(59, 157, 248, 0.14)',
		},
		{
			label: '调用成功率',
			value: `${(errorRate.rate * 100).toFixed(1)}%`,
			description: failedRequests > 0 ? `${formatNumber(failedRequests)} 次未成功` : '本月调用稳定',
			badge: '本月',
			icon: 'checkCircle',
			color: errorRate.rate >= 0.99 ? '#18b886' : errorRate.rate >= 0.95 ? '#f2aa35' : '#ef6a6a',
			soft: errorRate.rate >= 0.99 ? 'rgba(24, 184, 134, 0.14)' : errorRate.rate >= 0.95 ? 'rgba(242, 170, 53, 0.14)' : 'rgba(239, 106, 106, 0.14)',
		},
		{
			label: '平均响应',
			value: formatMs(latency.avg_ms),
			description: `首 Token ${formatMs(latency.avg_first_token_ms)}`,
			badge: '本月',
			icon: 'clock',
			color: '#22b8b4',
			soft: 'rgba(34, 184, 180, 0.14)',
		},
	]
})

const quotaPercent = computed(() => Math.min(Math.max(overviewData.value?.quota?.usage_percent || 0, 0), 100))

const quotaRemaining = computed(() => {
	const quota = overviewData.value?.quota
	return quota ? Math.max(quota.quota_limit - quota.quota_used, 0) : 0
})

const quotaState = computed(() => {
	const percent = overviewData.value?.quota?.usage_percent || 0
	if (percent >= 90) return { label: '即将用尽', tone: 'quota-danger' }
	if (percent >= 70) return { label: '需要关注', tone: 'quota-warning' }
	return { label: '额度充足', tone: 'quota-healthy' }
})

const latencyItems = computed(() => {
	if (!overviewData.value) return []
	const latency = overviewData.value.latency
	return [
		{ label: '平均响应', value: formatMs(latency.avg_ms) },
		{ label: '首 Token', value: formatMs(latency.avg_first_token_ms) },
		{ label: 'P95', value: formatMs(latency.p95_ms) },
		{ label: 'P99', value: formatMs(latency.p99_ms) },
	]
})

const topApiKeys = computed(() => apiKeyData.value.slice(0, 6))
const maxApiKeyRequests = computed(() => Math.max(...topApiKeys.value.map((item) => item.requests), 1))
const topModels = computed(() => modelData.value.slice(0, 6))
const maxModelCost = computed(() => Math.max(...topModels.value.map((item) => Number(item.total_cost || 0)), 0.000001))

async function fetchOverview() {
	loading.value = true
	try {
		const response: any = await request.get('/tenant/personal-dashboard')
		overviewData.value = response.data?.data || null
	} catch {
		overviewData.value = null
	} finally {
		loading.value = false
	}
}

async function fetchCharts() {
	chartsLoading.value = true
	try {
		const [trendResponse, modelResponse, keyResponse]: any[] = await Promise.all([
			request.get('/tenant/personal-dashboard/trends', { params: { days: selectedDays.value } }),
			request.get('/tenant/personal-dashboard/models', { params: { days: selectedDays.value } }),
			request.get('/tenant/personal-dashboard/api-key-usage', { params: { days: selectedDays.value } }),
		])
		trendData.value = ensureArray(trendResponse.data?.data?.list)
		modelData.value = ensureArray(modelResponse.data?.data?.list)
		apiKeyData.value = ensureArray(keyResponse.data?.data?.list)
	} catch {
		trendData.value = []
		modelData.value = []
		apiKeyData.value = []
	} finally {
		chartsLoading.value = false
	}
}

function refreshAll() {
	void fetchOverview()
	void fetchCharts()
}

watch(selectedDays, () => {
	void fetchCharts()
})

onMounted(refreshAll)
</script>

<template>
	<div class="personal-dashboard">
		<section class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<p class="mb-1 text-sm font-medium text-primary-500">{{ authStore.tenant?.name || '个人工作台' }}</p>
				<h1 class="text-2xl font-bold text-slate-900 md:text-[28px]">
					{{ greeting }}，{{ authStore.user?.username || '用户' }}
				</h1>
				<p class="mt-1 text-sm text-slate-400">{{ formattedDate }} · 这是你的 API 使用概况</p>
			</div>
			<button class="personal-refresh" :disabled="loading || chartsLoading" @click="refreshAll">
				<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading || chartsLoading }" />
				刷新数据
			</button>
		</section>

		<section v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<div v-for="index in 4" :key="index" class="stat-card h-[148px]">
				<div class="flex items-center justify-between">
					<div class="skeleton h-10 w-10 rounded-xl"></div>
					<div class="skeleton h-5 w-14 rounded-full"></div>
				</div>
				<div class="skeleton mt-4 h-8 w-28"></div>
				<div class="skeleton mt-3 h-3.5 w-36"></div>
			</div>
		</section>

		<template v-else-if="overviewData">
			<section class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
				<article
					v-for="stat in coreStats"
					:key="stat.label"
					class="stat-card personal-metric-card"
					:style="{ '--metric-color': stat.color, '--metric-soft': stat.soft }"
				>
					<div class="personal-metric-accent" aria-hidden="true"></div>
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-3">
							<div class="personal-metric-icon"><Icon :name="stat.icon" size="md" /></div>
							<p class="truncate text-sm font-semibold text-slate-600">{{ stat.label }}</p>
						</div>
						<span class="personal-metric-badge">{{ stat.badge }}</span>
					</div>
					<p class="personal-metric-value" :title="stat.value">{{ stat.value }}</p>
					<div class="personal-metric-detail">
						<span class="personal-metric-detail-dot" aria-hidden="true"></span>
						<span class="truncate">{{ stat.description }}</span>
					</div>
				</article>
			</section>

			<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
				<TokenTrendChart
					:data="trendData"
					:loading="chartsLoading"
					:days="selectedDays"
					@change-days="selectedDays = $event"
				/>

				<div class="card quota-card p-5 sm:p-6">
					<div class="flex items-start justify-between gap-4">
						<div class="flex items-center gap-3">
							<div class="section-icon quota-icon"><Icon name="shield" size="md" /></div>
							<div>
								<h2 class="text-base font-semibold text-slate-900">个人额度</h2>
								<p class="mt-0.5 text-xs text-slate-400">消费上限与当前使用状态</p>
							</div>
						</div>
						<span v-if="overviewData.quota" class="quota-status" :class="quotaState.tone">{{ quotaState.label }}</span>
					</div>

					<template v-if="overviewData.quota">
						<div class="mt-7 flex items-end justify-between gap-4">
							<div>
								<p class="text-xs font-medium text-slate-400">剩余额度</p>
								<p class="mt-1 text-3xl font-bold tabular-nums text-slate-900">{{ formatCost(quotaRemaining) }}</p>
							</div>
							<p class="text-right text-sm font-semibold tabular-nums text-slate-600">{{ overviewData.quota.usage_percent.toFixed(1) }}%</p>
						</div>
						<div class="quota-progress mt-4">
							<div class="quota-progress-bar" :class="quotaState.tone" :style="{ width: `${quotaPercent}%` }"></div>
						</div>
						<div class="mt-3 flex items-center justify-between text-xs text-slate-400">
							<span>已用 {{ formatCost(overviewData.quota.quota_used) }}</span>
							<span>总额 {{ formatCost(overviewData.quota.quota_limit) }}</span>
						</div>
						<div class="quota-meta mt-6">
							<div>
								<p class="text-[11px] text-slate-400">额度周期</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ overviewData.quota.period || overviewData.quota.quota_type }}</p>
							</div>
							<div class="text-right">
								<p class="text-[11px] text-slate-400">下次重置</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ formatResetAt(overviewData.quota.next_reset_at) }}</p>
							</div>
						</div>
					</template>

					<div v-else class="flex min-h-[270px] flex-col justify-between pt-7">
						<div>
							<p class="text-xs font-medium text-slate-400">本月消费</p>
							<p class="mt-1 text-3xl font-bold tabular-nums text-slate-900">{{ formatCost(overviewData.month.total_cost) }}</p>
							<p class="mt-2 text-sm text-slate-500">当前账户未设置个人额度限制</p>
						</div>
						<div class="quota-meta">
							<div>
								<p class="text-[11px] text-slate-400">本月请求</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ formatNumber(overviewData.month.requests) }} 次</p>
							</div>
							<div class="text-right">
								<p class="text-[11px] text-slate-400">本月 Token</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ formatNumber(overviewData.month.input_tokens + overviewData.month.output_tokens) }}</p>
							</div>
						</div>
					</div>
				</div>
			</section>

			<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
				<div class="card p-5 sm:p-6">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3">
							<div class="section-icon quality-icon"><Icon name="checkCircle" size="md" /></div>
							<div>
								<h2 class="text-base font-semibold text-slate-900">运行质量</h2>
								<p class="mt-0.5 text-xs text-slate-400">本月请求可靠性与性能</p>
							</div>
						</div>
						<strong class="text-lg font-bold tabular-nums" :class="overviewData.error_rate.rate >= 0.99 ? 'text-emerald-600' : overviewData.error_rate.rate >= 0.95 ? 'text-amber-600' : 'text-red-500'">
							{{ (overviewData.error_rate.rate * 100).toFixed(1) }}%
						</strong>
					</div>

					<div class="mt-5 grid gap-5 md:grid-cols-[minmax(0,.8fr)_minmax(0,1.2fr)]">
						<div class="grid grid-cols-2 gap-3">
							<div v-for="item in latencyItems" :key="item.label" class="quality-metric">
								<p class="text-[11px] text-slate-400">{{ item.label }}</p>
								<p class="mt-1 text-base font-bold tabular-nums text-slate-700">{{ item.value }}</p>
							</div>
						</div>

						<div class="border-t border-slate-100 pt-4 md:border-l md:border-t-0 md:pl-5 md:pt-0">
							<div class="mb-3 flex items-center justify-between">
								<p class="text-xs font-semibold text-slate-600">请求方式</p>
								<p class="text-[11px] text-slate-400">{{ formatNumber(overviewData.error_rate.total) }} 次</p>
							</div>
							<div v-if="overviewData.request_types.length" class="space-y-3">
								<div v-for="item in overviewData.request_types" :key="item.type">
									<div class="mb-1.5 flex items-center justify-between text-xs">
										<span class="text-slate-500">{{ item.label }}</span>
										<span class="font-semibold tabular-nums text-slate-600">{{ item.percentage.toFixed(1) }}%</span>
									</div>
									<div class="mini-progress"><span :style="{ width: `${Math.min(item.percentage, 100)}%` }"></span></div>
								</div>
							</div>
							<p v-else class="py-4 text-center text-xs text-slate-400">暂无请求数据</p>
						</div>
					</div>

					<div class="cache-strip mt-5">
						<div class="flex items-center gap-2">
							<Icon name="bolt" size="sm" class="text-cyan-500" />
							<span class="text-xs font-medium text-slate-600">缓存命中率</span>
						</div>
						<div class="text-right">
							<strong class="text-sm font-bold tabular-nums text-cyan-600">{{ (overviewData.cache.hit_ratio * 100).toFixed(1) }}%</strong>
							<p class="mt-0.5 text-[10px] text-slate-400">读取 {{ formatNumber(overviewData.cache.cache_read_tokens) }} Token</p>
						</div>
					</div>
				</div>

				<ModelDistChart :data="modelData" :loading="chartsLoading" />
			</section>

			<section class="grid grid-cols-1 gap-5 xl:grid-cols-2">
				<div class="card overflow-hidden">
					<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
						<div>
							<h2 class="text-base font-semibold text-slate-900">API Key 用量</h2>
							<p class="mt-0.5 text-xs text-slate-400">近 {{ selectedDays }} 天个人密钥调用排行</p>
						</div>
						<Icon name="key" size="md" class="text-blue-400" />
					</div>
					<div v-if="chartsLoading" class="space-y-3 p-5 sm:p-6">
						<div v-for="index in 4" :key="index" class="skeleton h-14 rounded-xl"></div>
					</div>
					<div v-else-if="topApiKeys.length === 0" class="flex min-h-64 flex-col items-center justify-center px-6 text-center">
						<div class="empty-icon"><Icon name="key" size="lg" /></div>
						<p class="mt-3 text-sm font-medium text-slate-600">暂无密钥用量</p>
						<p class="mt-1 text-xs text-slate-400">使用个人 API Key 调用后将在这里展示</p>
					</div>
					<div v-else class="divide-y divide-slate-100/80 px-5 sm:px-6">
						<div v-for="item in topApiKeys" :key="item.api_key_id" class="usage-row">
							<div class="usage-icon key-usage-icon"><Icon name="key" size="sm" /></div>
							<div class="min-w-0 flex-1">
								<div class="flex items-center justify-between gap-3">
									<p class="truncate text-sm font-semibold text-slate-700">{{ item.key_name }}</p>
									<span class="text-sm font-bold tabular-nums text-slate-700">{{ formatCost(item.total_cost) }}</span>
								</div>
								<div class="mt-1.5 flex items-center gap-3">
									<div class="usage-progress"><span :style="{ width: `${(item.requests / maxApiKeyRequests) * 100}%` }"></span></div>
									<span class="flex-shrink-0 text-[11px] text-slate-400">{{ formatNumber(item.requests) }} 次 · {{ item.key_prefix }}***</span>
								</div>
							</div>
						</div>
					</div>
				</div>

				<div class="card overflow-hidden">
					<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
						<div>
							<h2 class="text-base font-semibold text-slate-900">模型用量明细</h2>
							<p class="mt-0.5 text-xs text-slate-400">近 {{ selectedDays }} 天请求与费用排行</p>
						</div>
						<Icon name="cube" size="md" class="text-violet-400" />
					</div>
					<div v-if="chartsLoading" class="space-y-3 p-5 sm:p-6">
						<div v-for="index in 4" :key="index" class="skeleton h-14 rounded-xl"></div>
					</div>
					<div v-else-if="topModels.length === 0" class="flex min-h-64 flex-col items-center justify-center px-6 text-center">
						<div class="empty-icon model-empty-icon"><Icon name="cube" size="lg" /></div>
						<p class="mt-3 text-sm font-medium text-slate-600">暂无模型用量</p>
						<p class="mt-1 text-xs text-slate-400">产生模型调用后将在这里展示</p>
					</div>
					<div v-else class="divide-y divide-slate-100/80 px-5 sm:px-6">
						<div v-for="item in topModels" :key="item.model_name" class="usage-row">
							<div class="usage-icon model-usage-icon"><Icon name="cube" size="sm" /></div>
							<div class="min-w-0 flex-1">
								<div class="flex items-center justify-between gap-3">
									<p class="truncate text-sm font-semibold text-slate-700" :title="item.model_name">{{ item.model_name }}</p>
									<span class="text-sm font-bold tabular-nums text-emerald-600">{{ formatCost(item.total_cost) }}</span>
								</div>
								<div class="mt-1.5 flex items-center gap-3">
									<div class="usage-progress model-progress"><span :style="{ width: `${(item.total_cost / maxModelCost) * 100}%` }"></span></div>
									<span class="flex-shrink-0 text-[11px] text-slate-400">{{ formatNumber(item.requests) }} 次 · {{ formatNumber(item.input_tokens + item.output_tokens) }} Token</span>
								</div>
							</div>
						</div>
					</div>
				</div>
			</section>
		</template>

		<div v-else class="card">
			<div class="empty-state">
				<Icon name="chart" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无个人数据</p>
				<p class="empty-state-description">完成首次 API 调用后，这里会展示你的使用概况</p>
			</div>
		</div>
	</div>
</template>

<style scoped>
.personal-dashboard {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
}

.personal-refresh {
	display: inline-flex;
	height: 2.5rem;
	align-items: center;
	justify-content: center;
	gap: 0.5rem;
	padding: 0 1rem;
	border: 1px solid rgba(255, 255, 255, 0.88);
	border-radius: 9999px;
	background: rgba(255, 255, 255, 0.68);
	box-shadow: 0 8px 24px rgba(87, 102, 151, 0.08);
	color: #64748b;
	font-size: 0.8125rem;
	font-weight: 600;
	transition: all 180ms ease;
}

.personal-refresh:hover:not(:disabled) {
	transform: translateY(-1px);
	background: white;
	color: #6558ea;
}

.personal-refresh:disabled {
	cursor: wait;
	opacity: 0.65;
}

.personal-metric-card {
	min-height: 148px;
	padding: 1.125rem 1.25rem;
	transition: transform 220ms ease, box-shadow 220ms ease;
}

.personal-metric-card:hover {
	transform: translateY(-3px);
	box-shadow: 0 20px 45px rgba(81, 94, 143, 0.14);
}

.personal-metric-accent {
	position: absolute;
	top: 0;
	right: 1.25rem;
	left: 1.25rem;
	height: 2px;
	border-radius: 0 0 9999px 9999px;
	background: linear-gradient(90deg, transparent, var(--metric-color), transparent);
	opacity: 0.7;
}

.personal-metric-icon,
.section-icon,
.usage-icon,
.empty-icon {
	display: flex;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
}

.personal-metric-icon {
	height: 2.5rem;
	width: 2.5rem;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 0.75rem;
	background: var(--metric-soft);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
	color: var(--metric-color);
}

.personal-metric-badge {
	flex-shrink: 0;
	border: 1px solid color-mix(in srgb, var(--metric-color) 20%, transparent);
	border-radius: 9999px;
	background: var(--metric-soft);
	padding: 0.25rem 0.5rem;
	color: var(--metric-color);
	font-size: 0.625rem;
	font-weight: 700;
}

.personal-metric-value {
	margin-top: 0.875rem;
	overflow: hidden;
	color: #172033;
	font-size: 1.75rem;
	font-weight: 750;
	font-variant-numeric: tabular-nums;
	line-height: 1;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.personal-metric-detail {
	display: flex;
	min-width: 0;
	align-items: center;
	gap: 0.5rem;
	margin-top: 0.75rem;
	color: #94a3b8;
	font-size: 0.6875rem;
}

.personal-metric-detail-dot {
	height: 0.375rem;
	width: 0.375rem;
	flex-shrink: 0;
	border-radius: 9999px;
	background: var(--metric-color);
}

.quota-card {
	background:
		radial-gradient(circle at 100% 0, rgba(117, 104, 248, 0.11), transparent 34%),
		var(--glass-bg-strong);
}

.section-icon {
	height: 2.5rem;
	width: 2.5rem;
	border-radius: 0.8rem;
}

.quota-icon {
	background: rgba(117, 104, 248, 0.12);
	color: #7568f8;
}

.quality-icon {
	background: rgba(24, 184, 134, 0.12);
	color: #18a979;
}

.quota-status {
	flex-shrink: 0;
	border-radius: 9999px;
	padding: 0.3rem 0.6rem;
	font-size: 0.6875rem;
	font-weight: 700;
}

.quota-status.quota-healthy { background: #ecfdf5; color: #10b981; }
.quota-status.quota-warning { background: #fffbeb; color: #d97706; }
.quota-status.quota-danger { background: #fef2f2; color: #ef4444; }

.quota-progress,
.mini-progress,
.usage-progress {
	overflow: hidden;
	border-radius: 9999px;
	background: rgba(226, 232, 240, 0.7);
}

.quota-progress { height: 0.625rem; }
.quota-progress-bar { height: 100%; border-radius: inherit; transition: width 500ms ease; }
.quota-progress-bar.quota-healthy { background: linear-gradient(90deg, #2cc5c1, #22b88e); }
.quota-progress-bar.quota-warning { background: linear-gradient(90deg, #fbbf24, #f59e0b); }
.quota-progress-bar.quota-danger { background: linear-gradient(90deg, #fb7185, #ef4444); }

.quota-meta,
.cache-strip {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	border: 1px solid rgba(255, 255, 255, 0.86);
	border-radius: 1rem;
	background: rgba(255, 255, 255, 0.52);
	padding: 0.875rem 1rem;
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.quality-metric {
	border: 1px solid rgba(255, 255, 255, 0.84);
	border-radius: 1rem;
	background: rgba(255, 255, 255, 0.5);
	padding: 0.75rem 0.875rem;
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.mini-progress { height: 0.375rem; }
.mini-progress span {
	display: block;
	height: 100%;
	border-radius: inherit;
	background: linear-gradient(90deg, #42a4f5, #7568f8);
}

.cache-strip {
	background: rgba(236, 254, 255, 0.54);
}

.usage-row {
	display: flex;
	align-items: center;
	gap: 0.875rem;
	padding: 0.9rem 0;
}

.usage-icon {
	height: 2.25rem;
	width: 2.25rem;
	border-radius: 0.75rem;
}

.key-usage-icon {
	background: #eff6ff;
	color: #3b82f6;
}

.model-usage-icon {
	background: #f5f3ff;
	color: #8b5cf6;
}

.usage-progress {
	height: 0.3rem;
	min-width: 3rem;
	flex: 1;
}

.usage-progress span {
	display: block;
	height: 100%;
	border-radius: inherit;
	background: linear-gradient(90deg, #42a4f5, #7568f8);
}

.model-progress span {
	background: linear-gradient(90deg, #2cc5c1, #67d49d);
}

.empty-icon {
	height: 3.5rem;
	width: 3.5rem;
	border-radius: 1rem;
	background: #eff6ff;
	color: #60a5fa;
}

.model-empty-icon {
	background: #f5f3ff;
	color: #a78bfa;
}

@media (max-width: 420px) {
	.usage-progress {
		display: none;
	}
}

@media (prefers-reduced-motion: reduce) {
	.personal-refresh,
	.personal-metric-card,
	.quota-progress-bar {
		transition: none;
	}
}
</style>
