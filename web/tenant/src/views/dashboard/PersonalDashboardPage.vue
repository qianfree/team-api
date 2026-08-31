<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { formatBilling } from '@/composables/useCurrency'
import { formatNumber } from '@/utils/renderUtils'
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
	saved_cost: number
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

interface FailureItem {
	status: string
	model_name: string
	error_message: string
	created_at: string
}

interface OverviewData {
	today: DayStats
	month: DayStats
	error_rate: ErrorRate
	latency: Latency
	cache: Cache
	request_types: ReqTypeItem[]
	quota?: QuotaStatus
	recent_failures: FailureItem[]
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

// 金额格式化统一走本位币（formatBilling 内部读取响应式 displayCurrency，配置变化自动重渲染）
function formatCost(value: unknown): string {
	const number = Number(value) || 0
	if (number === 0 || number >= 1) return formatBilling(number, 2)
	if (number >= 0.01) return formatBilling(number, 4)
	return formatBilling(number, 6)
}

function formatMs(value: unknown): string {
	const number = Number(value) || 0
	if (number >= 1000) return `${(number / 1000).toFixed(2)}s`
	return `${Math.round(number)}ms`
}

function formatResetAt(value?: string): string {
	if (!value) return '按额度周期重置'
	const date = new Date(value.replace(' ', 'T'))
	if (Number.isNaN(date.getTime())) return value
	return `${date.getMonth() + 1} 月 ${date.getDate()} 日`
}

const FAILURE_LABELS: Record<string, string> = {
	error: '失败',
	timeout: '超时',
	cancelled: '已取消',
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

const quotaPercent = computed(() => Math.min(Math.max(overviewData.value?.quota?.usage_percent || 0, 0), 100))

const quotaRemaining = computed(() => {
	const quota = overviewData.value?.quota
	return quota ? Math.max(quota.quota_limit - quota.quota_used, 0) : 0
})

const quotaState = computed(() => {
	const percent = overviewData.value?.quota?.usage_percent || 0
	if (percent >= 90) return { label: '即将用尽', tone: 'crit' }
	if (percent >= 70) return { label: '需要留意', tone: 'warn' }
	return { label: '额度充足', tone: 'ok' }
})

const latencyItems = computed(() => {
	if (!overviewData.value) return []
	const latency = overviewData.value.latency
	return [
		{ label: 'P50', value: formatMs(latency.p50_ms) },
		{ label: 'P95', value: formatMs(latency.p95_ms) },
		{ label: 'P99', value: formatMs(latency.p99_ms) },
	]
})

const failures = computed(() => overviewData.value?.recent_failures || [])
const failedCount = computed(() => {
	const rate = overviewData.value?.error_rate
	return rate ? rate.error + rate.timeout + rate.cancelled : 0
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
				<p class="mt-1 text-sm text-slate-400">{{ formattedDate }} · 这里只统计你自己发起的调用</p>
			</div>
			<button class="personal-refresh" :disabled="loading || chartsLoading" @click="refreshAll">
				<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading || chartsLoading }" />
				刷新数据
			</button>
		</section>

		<section v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<div class="stat-card h-[240px] sm:col-span-2"></div>
			<div v-for="index in 2" :key="index" class="stat-card h-[148px]">
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
				<!-- 额度是使用者最高频的问题，升为首屏主体 -->
				<div class="card card-prominent quota-card sm:col-span-2">
					<div class="flex items-start justify-between gap-4">
						<div class="flex items-center gap-3">
							<div class="quota-icon"><Icon name="shield" size="md" /></div>
							<div>
								<h2 class="text-base font-semibold text-slate-900">
									{{ overviewData.quota ? '本期可用上限' : '未设置个人上限' }}
								</h2>
								<p class="mt-0.5 text-xs text-slate-400">
									{{ overviewData.quota ? '团队为你设定的消费控制线' : '你的调用直接由团队钱包结算' }}
								</p>
							</div>
						</div>
						<span v-if="overviewData.quota" class="quota-status" :class="`quota-${quotaState.tone}`">{{ quotaState.label }}</span>
						<span v-else class="quota-status quota-ok">不受限</span>
					</div>

					<template v-if="overviewData.quota">
						<div class="mt-6 flex items-end justify-between gap-4">
							<div>
								<p class="text-xs font-medium text-slate-400">剩余可用</p>
								<p class="mt-1 text-[2rem] font-bold leading-none tabular-nums text-slate-900">{{ formatCost(quotaRemaining) }}</p>
							</div>
							<p class="text-right text-sm font-semibold tabular-nums text-slate-600">{{ overviewData.quota.usage_percent.toFixed(1) }}%</p>
						</div>
						<div class="progress mt-4">
							<div class="progress-bar" :class="`bar-${quotaState.tone}`" :style="{ width: `${quotaPercent}%` }"></div>
						</div>
						<div class="mt-2.5 flex items-center justify-between text-xs text-slate-400">
							<span>已用 {{ formatCost(overviewData.quota.quota_used) }}</span>
							<span>上限 {{ formatCost(overviewData.quota.quota_limit) }}</span>
						</div>
						<!-- 语义澄清：额度是控制线不是钱包，超限只会暂停调用，不扣个人资金 -->
						<p class="quota-explain">
							这是团队给你划的消费控制线，不是你的钱包余额。用尽后调用会暂停，不会扣除你的个人资金 —— 需要更多额度请联系团队管理员。
						</p>
						<div class="quota-meta">
							<div>
								<p class="text-[11px] text-slate-400">额度周期</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">
									{{ overviewData.quota.period || overviewData.quota.quota_type }}
								</p>
							</div>
							<div class="text-right">
								<p class="text-[11px] text-slate-400">下次重置</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ formatResetAt(overviewData.quota.next_reset_at) }}</p>
							</div>
						</div>
					</template>

					<template v-else>
						<div class="mt-6">
							<p class="text-xs font-medium text-slate-400">本月我的消费</p>
							<p class="mt-1 text-[2rem] font-bold leading-none tabular-nums text-slate-900">{{ formatCost(overviewData.month.total_cost) }}</p>
						</div>
						<p class="quota-explain">
							团队没有为你设置消费控制线，你的每一次调用都直接从团队钱包扣款。
						</p>
						<div class="quota-meta">
							<div>
								<p class="text-[11px] text-slate-400">本月请求</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">{{ formatNumber(overviewData.month.requests) }} 次</p>
							</div>
							<div class="text-right">
								<p class="text-[11px] text-slate-400">本月 Token</p>
								<p class="mt-1 text-sm font-semibold text-slate-700">
									{{ formatNumber(overviewData.month.input_tokens + overviewData.month.output_tokens) }}
								</p>
							</div>
						</div>
					</template>
				</div>

				<article class="stat-card personal-metric-card" style="--metric-color: #f59e0b; --metric-soft: rgba(245, 158, 11, 0.14)">
					<div class="personal-metric-accent" aria-hidden="true"></div>
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-3">
							<div class="personal-metric-icon"><Icon name="currencyDollar" size="md" /></div>
							<p class="truncate text-sm font-semibold text-slate-600">今日消费</p>
						</div>
						<span class="personal-metric-badge">今天</span>
					</div>
					<p class="personal-metric-value">{{ formatCost(overviewData.today.total_cost) }}</p>
					<div class="personal-metric-detail">
						<span class="personal-metric-detail-dot" aria-hidden="true"></span>
						<span class="truncate">{{ formatNumber(overviewData.today.requests) }} 次调用</span>
					</div>
				</article>

				<article class="stat-card personal-metric-card" style="--metric-color: #3b82f6; --metric-soft: rgba(59, 130, 246, 0.14)">
					<div class="personal-metric-accent" aria-hidden="true"></div>
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-3">
							<div class="personal-metric-icon"><Icon name="chart" size="md" /></div>
							<p class="truncate text-sm font-semibold text-slate-600">本月消费</p>
						</div>
						<span class="personal-metric-badge">本月</span>
					</div>
					<p class="personal-metric-value">{{ formatCost(overviewData.month.total_cost) }}</p>
					<div class="personal-metric-detail">
						<span class="personal-metric-detail-dot" aria-hidden="true"></span>
						<span class="truncate">{{ formatNumber(overviewData.month.requests) }} 次调用</span>
					</div>
				</article>
			</section>

			<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
				<TokenTrendChart
					:data="trendData"
					:loading="chartsLoading"
					:days="selectedDays"
					title="我的消费趋势"
					:subtitle="`近 ${selectedDays} 天，按天聚合`"
					@change-days="selectedDays = $event"
				/>

				<!-- 看到成功率之后，使用者的下一个动作一定是「哪几次失败了」 -->
				<div class="card card-prominent flex flex-col">
					<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
						<div>
							<h2 class="text-base font-semibold text-slate-900">最近失败的调用</h2>
							<p class="mt-0.5 text-xs text-slate-400">
								本月 {{ formatNumber(failedCount) }} 次未成功，成功率 {{ (overviewData.error_rate.rate * 100).toFixed(1) }}%
							</p>
						</div>
						<Icon name="exclamationTriangle" size="md" class="flex-shrink-0 text-amber-400" />
					</div>

					<div v-if="failures.length === 0" class="flex flex-1 flex-col items-center justify-center px-6 py-10 text-center">
						<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-emerald-50 text-emerald-500">
							<Icon name="checkCircle" size="lg" />
						</div>
						<p class="mt-3 text-sm font-medium text-slate-600">本月没有失败的调用</p>
						<p class="mt-1 text-xs text-slate-400">一切正常</p>
					</div>
					<div v-else class="divide-y divide-slate-100/80 px-5 sm:px-6">
						<div v-for="(item, index) in failures" :key="index" class="flex gap-3 py-3">
							<span class="failure-badge">{{ FAILURE_LABELS[item.status] || item.status }}</span>
							<div class="min-w-0 flex-1">
								<p class="truncate font-mono text-xs text-slate-700" :title="item.error_message">
									{{ item.error_message || '上游未返回错误信息' }}
								</p>
								<p class="mt-0.5 text-[11px] text-slate-400">{{ item.created_at }} · {{ item.model_name }}</p>
							</div>
						</div>
					</div>

					<div v-if="failures.length" class="mt-auto border-t border-slate-100/80 px-5 py-3.5 sm:px-6">
						<router-link to="/tenant/usage-logs" class="text-xs font-medium text-primary-600 transition-colors hover:text-primary-700">
							在用量日志里查看全部 →
						</router-link>
					</div>
				</div>
			</section>

			<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
				<div class="card card-prominent p-5 sm:p-6">
					<div class="flex items-center justify-between">
						<div class="flex items-center gap-3">
							<div class="section-icon quality-icon"><Icon name="checkCircle" size="md" /></div>
							<div>
								<h2 class="text-base font-semibold text-slate-900">运行质量</h2>
								<p class="mt-0.5 text-xs text-slate-400">本月请求可靠性与性能</p>
							</div>
						</div>
						<strong
							class="text-lg font-bold tabular-nums"
							:class="overviewData.error_rate.rate >= 0.99 ? 'text-emerald-600' : overviewData.error_rate.rate >= 0.95 ? 'text-amber-600' : 'text-red-500'"
						>
							{{ (overviewData.error_rate.rate * 100).toFixed(1) }}%
						</strong>
					</div>

					<div class="mt-5 grid gap-5 md:grid-cols-[minmax(0,.85fr)_minmax(0,1.15fr)]">
						<div>
							<div class="grid grid-cols-2 gap-3">
								<div class="quality-metric">
									<p class="text-[11px] text-slate-400">平均响应</p>
									<p class="mt-1 text-base font-bold tabular-nums text-slate-700">{{ formatMs(overviewData.latency.avg_ms) }}</p>
								</div>
								<div class="quality-metric">
									<p class="text-[11px] text-slate-400">首 Token</p>
									<p class="mt-1 text-base font-bold tabular-nums text-slate-700">{{ formatMs(overviewData.latency.avg_first_token_ms) }}</p>
								</div>
							</div>
							<!-- 分位数对普通成员偏技术，默认收起，排障时再展开 -->
							<details class="latency-details">
								<summary>展开延迟分位数</summary>
								<div class="mt-2.5 grid grid-cols-3 gap-2">
									<div v-for="item in latencyItems" :key="item.label" class="quality-metric">
										<p class="text-[11px] text-slate-400">{{ item.label }}</p>
										<p class="mt-1 text-sm font-bold tabular-nums text-slate-700">{{ item.value }}</p>
									</div>
								</div>
							</details>
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
							<Icon name="bolt" size="sm" class="flex-shrink-0 text-cyan-500" />
							<div>
								<span class="text-xs font-medium text-slate-600">缓存命中率 {{ (overviewData.cache.hit_ratio * 100).toFixed(1) }}%</span>
								<p class="mt-0.5 text-[11px] text-slate-400">
									已命中 {{ formatNumber(overviewData.cache.cache_read_tokens) }} Token，约省下 {{ formatCost(overviewData.cache.saved_cost) }}
								</p>
							</div>
						</div>
					</div>
				</div>

				<ModelDistChart :data="modelData" :loading="chartsLoading" />
			</section>

			<section class="grid grid-cols-1 gap-5 xl:grid-cols-2">
				<div class="card card-prominent overflow-hidden">
					<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
						<div>
							<h2 class="text-base font-semibold text-slate-900">我的 API Key 用量</h2>
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

				<div class="card card-prominent overflow-hidden">
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
									<span class="flex-shrink-0 text-[11px] text-slate-400">
										{{ formatNumber(item.requests) }} 次 · {{ formatNumber(item.input_tokens + item.output_tokens) }} Token
									</span>
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
	color: #0d9488;
}

.personal-refresh:disabled {
	cursor: wait;
	opacity: 0.65;
}

/* ── 额度主卡 ── */
.quota-card {
	display: flex;
	flex-direction: column;
	padding: 1.25rem 1.375rem 1.375rem;
}

.quota-icon {
	display: flex;
	height: 2.5rem;
	width: 2.5rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 0.75rem;
	background: rgba(13, 148, 136, 0.14);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
	color: #0d9488;
}

.quota-status {
	display: inline-flex;
	flex-shrink: 0;
	align-items: center;
	gap: 0.35rem;
	border-radius: 9999px;
	padding: 0.3rem 0.6rem;
	font-size: 0.6875rem;
	font-weight: 700;
}

.quota-status::before {
	height: 0.4rem;
	width: 0.4rem;
	border-radius: 9999px;
	background: currentColor;
	content: '';
}

.quota-ok {
	background: #ecfdf5;
	color: #10b981;
}

.quota-warn {
	background: #fffbeb;
	color: #f59e0b;
}

.quota-crit {
	background: #fef2f2;
	color: #ef4444;
}

.bar-ok {
	background: linear-gradient(90deg, #2dd4bf, #0d9488);
}

.bar-warn {
	background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.bar-crit {
	background: linear-gradient(90deg, #f87171, #ef4444);
}

.quota-explain {
	margin-top: 1rem;
	border-radius: 0.6rem;
	background: rgba(148, 163, 184, 0.1);
	padding: 0.6rem 0.75rem;
	color: #64748b;
	font-size: 0.6875rem;
	line-height: 1.65;
}

.quota-meta {
	display: flex;
	justify-content: space-between;
	gap: 1rem;
	margin-top: auto;
	padding-top: 1rem;
}

/* ── 指标卡 ── */
.personal-metric-card {
	position: relative;
	min-height: 148px;
	overflow: hidden;
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

.personal-metric-icon {
	display: flex;
	height: 2.5rem;
	width: 2.5rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
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
	font-size: 1.65rem;
	font-weight: 750;
	font-variant-numeric: tabular-nums;
	line-height: 1.1;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.personal-metric-detail {
	display: flex;
	min-width: 0;
	align-items: center;
	gap: 0.5rem;
	margin-top: 0.65rem;
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

/* ── 运行质量 ── */
.section-icon {
	display: flex;
	height: 2.5rem;
	width: 2.5rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border: 1px solid rgba(255, 255, 255, 0.82);
	border-radius: 0.75rem;
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.quality-icon {
	background: rgba(16, 185, 129, 0.14);
	color: #10b981;
}

.quality-metric {
	border: 1px solid rgba(255, 255, 255, 0.85);
	border-radius: 0.85rem;
	background: rgba(255, 255, 255, 0.6);
	padding: 0.7rem 0.8rem;
}

.latency-details {
	margin-top: 0.7rem;
}

.latency-details > summary {
	color: #0d9488;
	cursor: pointer;
	font-size: 0.75rem;
	font-weight: 620;
	list-style: none;
}

.latency-details > summary::-webkit-details-marker {
	display: none;
}

.latency-details > summary::after {
	content: ' ▾';
}

.latency-details[open] > summary::after {
	content: ' ▴';
}

.mini-progress {
	height: 0.35rem;
	overflow: hidden;
	border-radius: 9999px;
	background: rgba(148, 163, 184, 0.2);
}

.mini-progress > span {
	display: block;
	height: 100%;
	border-radius: 9999px;
	background: linear-gradient(90deg, #2dd4bf, #0d9488);
}

.cache-strip {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	border-radius: 0.8rem;
	background: rgba(6, 182, 212, 0.08);
	padding: 0.7rem 0.85rem;
}

/* ── 失败列表 ── */
.failure-badge {
	align-self: flex-start;
	flex-shrink: 0;
	border-radius: 0.35rem;
	background: #fef2f2;
	padding: 0.15rem 0.42rem;
	color: #ef4444;
	font-size: 0.625rem;
	font-weight: 700;
	white-space: nowrap;
}

/* ── 用量列表 ── */
.usage-row {
	display: grid;
	grid-template-columns: auto minmax(0, 1fr);
	align-items: center;
	gap: 0.75rem;
	padding: 0.8rem 0;
}

.usage-icon {
	display: flex;
	height: 2rem;
	width: 2rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 0.65rem;
}

.key-usage-icon {
	background: rgba(59, 130, 246, 0.13);
	color: #3b82f6;
}

.model-usage-icon {
	background: rgba(139, 92, 246, 0.13);
	color: #8b5cf6;
}

.usage-progress {
	height: 0.35rem;
	flex: 1;
	overflow: hidden;
	border-radius: 9999px;
	background: rgba(148, 163, 184, 0.2);
}

.usage-progress > span {
	display: block;
	height: 100%;
	border-radius: 9999px;
	background: #3b82f6;
}

.model-progress > span {
	background: #8b5cf6;
}

.empty-icon {
	display: flex;
	height: 3.5rem;
	width: 3.5rem;
	align-items: center;
	justify-content: center;
	border-radius: 1rem;
	background: rgba(59, 130, 246, 0.1);
	color: #60a5fa;
}

.model-empty-icon {
	background: rgba(139, 92, 246, 0.1);
	color: #a78bfa;
}

@media (prefers-reduced-motion: reduce) {
	.personal-metric-card,
	.personal-refresh {
		animation: none;
		transition: none;
	}
}
</style>
