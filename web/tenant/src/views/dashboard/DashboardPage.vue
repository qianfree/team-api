<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

interface WalletInfo {
	balance: number
	frozen_balance: number
	available: number
	warning_threshold: number
}

interface CostTrend {
	current_cost: number
	previous_cost: number
	delta_percent: number
	has_previous: boolean
}

interface WasteStat {
	wasted_cost: number
	share_percent: number
	failed_requests: number
	retried_requests: number
}

interface DashboardData {
	today: DayStats | null
	month: DayStats | null
	wallet: WalletInfo | null
	rpm: number
	tpm: number
	active_keys: number
	member_count: number
	cost_trend: CostTrend
	waste: WasteStat
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

interface MemberUsageItem {
	user_id: number
	username: string
	display_name: string
	requests: number
	input_tokens: number
	output_tokens: number
	total_cost: number
	delta_percent: number
	has_previous: boolean
	quota_percent: number
	has_quota: boolean
}

interface PredictionData {
	daily_avg_cost: number
	available_balance: number
	will_exhaust: boolean
	days_until_exhaust?: number
	exhaust_date?: string
	message?: string
}

interface MemberAlert {
	id: number
	username: string
	display_name: string
	quota_limit: number
	quota_used: number
	usage_percent: number
	next_reset_at?: string
}

interface ProjectAlert {
	id: number
	name: string
	budget: number
	used: number
	usage_percent: number
}

interface TeamHealth {
	total_requests: number
	success_rate: number
	p95_ms: number
	avg_first_token_ms: number
	cache_hit_ratio: number
	cache_read_tokens: number
}

interface ProjectBudgetItem {
	id: number
	name: string
	budget: number
	used: number
	usage_percent: number
	has_budget: boolean
}

const authStore = useTenantAuthStore()
const loading = ref(false)
const chartsLoading = ref(false)
const memberUsageLoading = ref(false)
const alertsLoading = ref(false)
const healthLoading = ref(false)
const budgetLoading = ref(false)
// 管理者看趋势默认看 30 天：经营视角关心的是月度走势，不是最近一周
const selectedDays = ref(30)
const dashboardData = ref<DashboardData | null>(null)
const trendData = ref<TrendPoint[]>([])
const modelData = ref<ModelItem[]>([])
const memberUsageData = ref<MemberUsageItem[]>([])
const memberPrevAvailable = ref(true)
const predictionData = ref<PredictionData | null>(null)
const memberAlerts = ref<MemberAlert[]>([])
const projectAlerts = ref<ProjectAlert[]>([])
const teamHealth = ref<TeamHealth | null>(null)
const projectBudgets = ref<ProjectBudgetItem[]>([])

const safeWallet = (wallet: WalletInfo | null | undefined): WalletInfo =>
	wallet || { balance: 0, frozen_balance: 0, available: 0, warning_threshold: 0 }

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

const greeting = computed(() => {
	const hour = new Date().getHours()
	if (hour < 6) return '夜深了'
	if (hour < 12) return '早上好'
	if (hour < 18) return '下午好'
	return '晚上好'
})

const formattedDate = computed(() =>
	new Intl.DateTimeFormat('zh-CN', {
		year: 'numeric',
		month: 'long',
		day: 'numeric',
		weekday: 'long',
	}).format(new Date()),
)

function toneOf(percent: number): 'ok' | 'warn' | 'crit' {
	if (percent >= 90) return 'crit'
	if (percent >= 70) return 'warn'
	return 'ok'
}

// 首屏四卡：管理者先看钱——余额、套餐额度、消费与环比、还能撑几天。
// 请求数与 Token 降级为脚注，不再占据主位。
const coreStats = computed(() => {
	if (!dashboardData.value) return []
	const data = dashboardData.value
	const wallet = safeWallet(data.wallet)
	const trend = data.cost_trend
	const prediction = predictionData.value
	const cards: Array<Record<string, any>> = []

	cards.push({
		key: 'balance',
		label: '钱包可用余额',
		value: formatCost(wallet.available),
		sub: `另有 ${formatCost(wallet.frozen_balance)} 处于预扣冻结`,
		icon: 'wallet',
		color: '#0d9488',
		soft: 'rgba(13, 148, 136, 0.13)',
	})

	cards.push({
		key: 'today',
		label: '今日消费',
		value: formatCost(data.today?.total_cost ?? 0),
		sub: `${formatNumber(data.today?.requests ?? 0)} 次调用`,
		icon: 'play',
		color: '#8b5cf6',
		soft: 'rgba(139, 92, 246, 0.13)',
	})

	cards.push({
		key: 'cost',
		label: '本月消费',
		value: formatCost(trend?.current_cost ?? data.month?.total_cost ?? 0),
		sub: trend?.has_previous
			? `${formatNumber(data.month?.requests ?? 0)} 次调用 · 上月同期 ${formatCost(trend.previous_cost)}`
			: `${formatNumber(data.month?.requests ?? 0)} 次调用 · 上月同期无数据`,
		icon: 'currencyDollar',
		color: '#f59e0b',
		soft: 'rgba(245, 158, 11, 0.13)',
		delta: trend?.has_previous
			? {
				text: `${trend.delta_percent > 0 ? '↑' : trend.delta_percent < 0 ? '↓' : '→'} ${Math.abs(trend.delta_percent).toFixed(1)}%`,
				tone: trend.delta_percent > 0 ? 'up' : trend.delta_percent < 0 ? 'down' : 'flat',
			}
			: null,
	})

	let forecastValue = '充足'
	let forecastSub = prediction?.message || '按当前速度余额稳定'
	let forecastColor = '#10b981'
	let forecastSoft = 'rgba(16, 185, 129, 0.13)'
	let forecastState = ''
	if (prediction?.will_exhaust && prediction.days_until_exhaust !== undefined) {
		forecastValue = `${prediction.days_until_exhaust} 天`
		forecastSub = `按日均 ${formatCost(prediction.daily_avg_cost)} 估算 · 预计 ${prediction.exhaust_date || '近期'} 耗尽`
		forecastState = prediction.days_until_exhaust <= 7 ? 'crit' : 'warn'
		forecastColor = prediction.days_until_exhaust <= 7 ? '#ef4444' : '#f59e0b'
		forecastSoft = prediction.days_until_exhaust <= 7 ? 'rgba(239, 68, 68, 0.13)' : 'rgba(245, 158, 11, 0.13)'
	}
	cards.push({
		key: 'forecast',
		label: '余额可支撑',
		value: forecastValue,
		sub: forecastSub,
		icon: 'hourglass',
		color: forecastColor,
		soft: forecastSoft,
		state: forecastState,
	})

	return cards
})

const alertItems = computed(() => {
	const projects = projectAlerts.value.map((item) => ({
		id: `project-${item.id}`,
		name: item.name,
		percent: item.usage_percent,
		type: '项目预算',
		detail: `${formatCost(item.used)} / ${formatCost(item.budget)}`,
	}))
	const members = memberAlerts.value.map((item) => ({
		id: `member-${item.id}`,
		name: item.display_name || item.username,
		percent: item.usage_percent,
		type: '成员额度',
		detail: `${formatCost(item.quota_used)} / ${formatCost(item.quota_limit)}`,
	}))
	return [...projects, ...members].sort((a, b) => b.percent - a.percent)
})

const visibleAlerts = computed(() => alertItems.value.slice(0, 4))

const budgetedProjects = computed(() => projectBudgets.value.filter((item) => item.has_budget))

function ensureArray<T>(value: unknown): T[] {
	return Array.isArray(value) ? value : []
}

async function fetchDashboard() {
	loading.value = true
	try {
		const response: any = await request.get('/tenant/dashboard')
		dashboardData.value = response.data?.data || null
	} catch {
		dashboardData.value = null
	} finally {
		loading.value = false
	}
}

async function fetchCharts() {
	chartsLoading.value = true
	try {
		const [trendResponse, modelResponse]: any[] = await Promise.all([
			request.get('/tenant/dashboard/token-trends', { params: { days: selectedDays.value } }),
			request.get('/tenant/dashboard/model-distribution', { params: { days: selectedDays.value } }),
		])
		trendData.value = ensureArray(trendResponse.data?.data?.list)
		modelData.value = ensureArray(modelResponse.data?.data?.list)
	} catch {
		trendData.value = []
		modelData.value = []
	} finally {
		chartsLoading.value = false
	}
}

async function fetchPrediction() {
	try {
		const response: any = await request.get('/tenant/dashboard/balance-prediction')
		predictionData.value = response.data?.data || null
	} catch {
		predictionData.value = null
	}
}

async function fetchAlerts() {
	alertsLoading.value = true
	try {
		const response: any = await request.get('/tenant/dashboard/budget-alerts')
		memberAlerts.value = ensureArray(response.data?.data?.members)
		projectAlerts.value = ensureArray(response.data?.data?.projects)
	} catch {
		memberAlerts.value = []
		projectAlerts.value = []
	} finally {
		alertsLoading.value = false
	}
}

async function fetchMemberUsage() {
	memberUsageLoading.value = true
	try {
		const response: any = await request.get('/tenant/dashboard/member-usage-ranking', {
			params: { days: selectedDays.value, limit: 8 },
		})
		memberUsageData.value = ensureArray(response.data?.data?.list)
		memberPrevAvailable.value = response.data?.data?.prev_available !== false
	} catch {
		memberUsageData.value = []
		memberPrevAvailable.value = false
	} finally {
		memberUsageLoading.value = false
	}
}

async function fetchTeamHealth() {
	healthLoading.value = true
	try {
		const response: any = await request.get('/tenant/dashboard/team-health')
		teamHealth.value = response.data?.data || null
	} catch {
		teamHealth.value = null
	} finally {
		healthLoading.value = false
	}
}

async function fetchProjectBudgets() {
	budgetLoading.value = true
	try {
		const response: any = await request.get('/tenant/dashboard/project-budget')
		projectBudgets.value = ensureArray(response.data?.data?.list)
	} catch {
		projectBudgets.value = []
	} finally {
		budgetLoading.value = false
	}
}

function refreshAll() {
	void fetchDashboard()
	void fetchCharts()
	void fetchPrediction()
	void fetchAlerts()
	void fetchMemberUsage()
	void fetchTeamHealth()
	void fetchProjectBudgets()
}

watch(selectedDays, () => {
	void fetchCharts()
	void fetchMemberUsage()
})

// 实时 RPM/TPM 依赖 60 秒滑动窗口，仅静默轮询 /tenant/dashboard 保持指标新鲜（30s 一次）
let dashboardTimer: ReturnType<typeof setInterval> | null = null

async function refreshDashboardSilently() {
	try {
		const response: any = await request.get('/tenant/dashboard')
		dashboardData.value = response.data?.data || dashboardData.value
	} catch {
		// 静默失败，保留上次数据
	}
}

onMounted(() => {
	refreshAll()
	dashboardTimer = setInterval(refreshDashboardSilently, 30000)
})

onBeforeUnmount(() => {
	if (dashboardTimer) clearInterval(dashboardTimer)
})
</script>

<template>
	<div class="dashboard-shell">
		<section class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<p class="mb-1 text-sm font-medium text-primary-500">{{ authStore.tenant?.name || '租户控制台' }}</p>
				<h1 class="text-2xl font-bold tracking-tight text-slate-900 md:text-[28px]">
					{{ greeting }}，{{ authStore.user?.username || 'Admin' }} <span class="inline-block origin-bottom-right animate-wave">👋</span>
				</h1>
				<p class="mt-1 text-sm text-slate-400">今天是 {{ formattedDate }}，以下是整个组织的用量与开支。</p>
			</div>
			<div class="flex items-center gap-3">
				<div v-if="dashboardData" class="rate-pill" title="最近 60 秒滑动窗口实时速率">
					<span class="rate-pill-label">RPM</span>
					<span class="rate-pill-value text-primary-600">{{ formatNumber(dashboardData.rpm) }}</span>
					<span class="rate-pill-sep"></span>
					<span class="rate-pill-label">TPM</span>
					<span class="rate-pill-value text-teal-600">{{ formatNumber(dashboardData.tpm) }}</span>
				</div>
				<button class="refresh-button" :disabled="loading || chartsLoading" @click="refreshAll">
					<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading || chartsLoading }" />
					刷新数据
				</button>
			</div>
		</section>

		<section v-if="loading" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<div v-for="index in 4" :key="index" class="metric-card">
				<div class="metric-head">
					<div class="skeleton h-4 w-24 rounded"></div>
					<div class="skeleton h-[1.85rem] w-[1.85rem] rounded-[0.6rem]"></div>
				</div>
				<div class="skeleton mt-[0.85rem] h-8 w-32 rounded"></div>
				<div class="skeleton mt-auto h-3.5 w-40 rounded"></div>
			</div>
		</section>

		<section v-else-if="dashboardData" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<article
				v-for="stat in coreStats"
				:key="stat.key"
				class="metric-card"
				:class="stat.state ? `metric-${stat.state}` : ''"
				:style="{ '--metric-color': stat.color, '--metric-soft': stat.soft }"
			>
				<div class="metric-head">
					<p class="metric-label">{{ stat.label }}</p>
					<span class="metric-icon"><Icon :name="stat.icon" size="sm" /></span>
				</div>

				<div class="metric-figure">
					<span class="metric-value" :title="stat.value">{{ stat.value }}</span>
					<span v-if="stat.delta" class="metric-delta" :class="`delta-${stat.delta.tone}`">{{ stat.delta.text }}</span>
				</div>

				<p class="metric-sub" :title="stat.sub">{{ stat.sub }}</p>
			</article>
		</section>

		<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
			<TokenTrendChart
				:data="trendData"
				:loading="chartsLoading"
				:days="selectedDays"
				title="消费趋势"
				:subtitle="`近 ${selectedDays} 天，按天聚合`"
				@change-days="selectedDays = $event"
			/>

			<div class="card card-prominent flex flex-col">
				<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
					<div>
						<h2 class="text-base font-semibold text-slate-900">需要处理</h2>
						<p class="mt-0.5 text-xs text-slate-400">超阈预算与被浪费掉的支出</p>
					</div>
					<span v-if="!alertsLoading" class="status-dot" :class="alertItems.length ? 'status-warning' : 'status-ok'">
						{{ alertItems.length ? `${alertItems.length} 项` : '运行正常' }}
					</span>
				</div>

				<div class="px-5 py-4 sm:px-6">
					<div v-if="alertsLoading" class="space-y-2">
						<div v-for="index in 3" :key="index" class="skeleton h-12 rounded-xl"></div>
					</div>
					<div v-else-if="visibleAlerts.length" class="space-y-2">
						<div
							v-for="alert in visibleAlerts"
							:key="alert.id"
							class="flex items-center gap-3 rounded-xl px-3 py-2.5"
							:class="alert.percent >= 90 ? 'bg-red-50/80' : 'bg-amber-50/80'"
						>
							<Icon
								name="exclamationTriangle"
								size="sm"
								class="flex-shrink-0"
								:class="alert.percent >= 90 ? 'text-red-500' : 'text-amber-500'"
							/>
							<div class="min-w-0 flex-1">
								<p class="truncate text-xs font-medium text-slate-700">{{ alert.name }}</p>
								<p class="text-[10px] text-slate-400">{{ alert.type }} · {{ alert.detail }}</p>
							</div>
							<span class="text-xs font-bold tabular-nums" :class="alert.percent >= 90 ? 'text-red-600' : 'text-amber-600'">
								{{ alert.percent.toFixed(0) }}%
							</span>
						</div>
					</div>
					<p v-else class="py-3 text-center text-xs text-slate-400">暂无超阈的项目预算或成员额度</p>
				</div>

				<div v-if="dashboardData" class="mt-auto border-t border-slate-100/80 px-5 py-4 sm:px-6">
					<div class="flex items-center justify-between gap-3">
						<div>
							<p class="text-sm font-semibold text-slate-700">无效支出</p>
							<p class="mt-0.5 text-[11px] text-slate-400">本月失败请求已产生的费用</p>
						</div>
						<div class="text-right">
							<p class="text-base font-bold tabular-nums text-red-500">{{ formatCost(dashboardData.waste.wasted_cost) }}</p>
							<p class="mt-0.5 text-[11px] text-slate-400">占本月 {{ dashboardData.waste.share_percent.toFixed(1) }}%</p>
						</div>
					</div>
					<div class="mt-3 grid grid-cols-2 gap-2.5">
						<div class="mini-tile">
							<p class="text-[11px] text-slate-400">失败请求</p>
							<p class="mt-0.5 text-sm font-bold tabular-nums text-slate-700">{{ formatNumber(dashboardData.waste.failed_requests) }}</p>
						</div>
						<div class="mini-tile">
							<p class="text-[11px] text-slate-400">触发重试</p>
							<p class="mt-0.5 text-sm font-bold tabular-nums text-slate-700">{{ formatNumber(dashboardData.waste.retried_requests) }}</p>
						</div>
					</div>
				</div>
			</div>
		</section>

		<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
			<div class="card card-prominent overflow-hidden">
				<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
					<div>
						<h2 class="text-base font-semibold text-slate-900">成员用量排行</h2>
						<p class="mt-0.5 text-xs text-slate-400">
							近 {{ selectedDays }} 天团队调用费用{{ memberPrevAvailable ? '，含环比' : '' }}
						</p>
					</div>
					<router-link to="/tenant/members" class="text-xs font-medium text-primary-600 transition-colors hover:text-primary-700">查看全部 →</router-link>
				</div>

				<div v-if="memberUsageLoading" class="space-y-3 p-5 sm:p-6">
					<div v-for="index in 4" :key="index" class="skeleton h-12 rounded-xl"></div>
				</div>
				<div v-else-if="memberUsageData.length === 0" class="flex min-h-64 flex-col items-center justify-center px-6 text-center">
					<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-400">
						<Icon name="users" size="lg" />
					</div>
					<p class="mt-3 text-sm font-medium text-slate-600">暂无成员用量</p>
					<p class="mt-1 text-xs text-slate-400">成员产生调用后将在这里展示</p>
				</div>
				<div v-else class="divide-y divide-slate-100/80 px-5 sm:px-6">
					<div
						v-for="(member, index) in memberUsageData"
						:key="member.user_id"
						class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 py-3.5"
					>
						<span
							class="flex h-7 w-7 items-center justify-center rounded-lg text-xs font-bold"
							:class="index < 3 ? 'bg-primary-50 text-primary-600' : 'bg-slate-50 text-slate-400'"
						>{{ index + 1 }}</span>
						<div class="flex min-w-0 items-center gap-3">
							<div class="member-avatar" :style="{ '--avatar-hue': `${(member.user_id * 43) % 360}` }">
								{{ (member.display_name || member.username).charAt(0).toUpperCase() }}
							</div>
							<div class="min-w-0">
								<div class="flex items-center gap-2">
									<p class="truncate text-sm font-medium text-slate-700">{{ member.display_name || member.username }}</p>
									<span v-if="member.has_quota && member.quota_percent >= 80" class="quota-flag">
										额度 {{ member.quota_percent.toFixed(0) }}%
									</span>
								</div>
								<p class="truncate text-xs text-slate-400">
									{{ formatNumber(member.requests) }} 次请求 · {{ formatNumber(member.input_tokens + member.output_tokens) }} Token
								</p>
							</div>
						</div>
						<div class="flex items-center gap-2.5">
							<span
								v-if="memberPrevAvailable && member.has_previous"
								class="trend-chip"
								:class="member.delta_percent > 5 ? 'chip-up' : member.delta_percent < -5 ? 'chip-down' : 'chip-flat'"
							>
								{{ member.delta_percent > 0 ? '+' : '' }}{{ member.delta_percent.toFixed(1) }}%
							</span>
							<span class="min-w-[5rem] text-right text-sm font-semibold tabular-nums text-slate-700">{{ formatCost(member.total_cost) }}</span>
						</div>
					</div>
				</div>
			</div>

			<ModelDistChart :data="modelData" :loading="chartsLoading" />
		</section>

		<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
			<div class="card card-prominent overflow-hidden">
				<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
					<div>
						<h2 class="text-base font-semibold text-slate-900">项目预算</h2>
						<!-- 预算是累计口径（与 CheckProjectBudget 同源），不随上方时间窗变化 -->
						<p class="mt-0.5 text-xs text-slate-400">各项目累计已用与预算上限</p>
					</div>
					<router-link to="/tenant/projects" class="text-xs font-medium text-primary-600 transition-colors hover:text-primary-700">项目管理 →</router-link>
				</div>

				<div v-if="budgetLoading" class="space-y-3 p-5 sm:p-6">
					<div v-for="index in 4" :key="index" class="skeleton h-12 rounded-xl"></div>
				</div>
				<div v-else-if="budgetedProjects.length === 0" class="flex min-h-56 flex-col items-center justify-center px-6 text-center">
					<div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-400">
						<Icon name="project" size="lg" />
					</div>
					<p class="mt-3 text-sm font-medium text-slate-600">暂无设置预算的项目</p>
					<p class="mt-1 text-xs text-slate-400">为项目设置预算后可在这里跟踪消耗</p>
				</div>
				<div v-else class="divide-y divide-slate-100/80 px-5 sm:px-6">
					<div v-for="project in budgetedProjects" :key="project.id" class="py-3.5">
						<div class="flex items-center justify-between gap-3">
							<span class="truncate text-sm font-medium text-slate-700">{{ project.name }}</span>
							<span class="flex-shrink-0 text-xs tabular-nums text-slate-500">
								{{ formatCost(project.used) }} / {{ formatCost(project.budget) }}
								<b
									class="ml-1.5"
									:class="project.usage_percent >= 90 ? 'text-red-500' : project.usage_percent >= 70 ? 'text-amber-500' : 'text-slate-600'"
								>{{ project.usage_percent.toFixed(0) }}%</b>
							</span>
						</div>
						<div class="progress mt-2">
							<div
								class="progress-bar"
								:class="`bar-${toneOf(project.usage_percent)}`"
								:style="{ width: `${Math.min(project.usage_percent, 100)}%` }"
							></div>
						</div>
					</div>
				</div>
			</div>

			<div class="card card-prominent p-5 sm:p-6">
				<div class="mb-4 flex items-center justify-between">
					<div>
						<h2 class="text-base font-semibold text-slate-900">团队运行质量</h2>
						<p class="mt-0.5 text-xs text-slate-400">本月整体可靠性与性能</p>
					</div>
					<span
						v-if="teamHealth"
						class="status-dot"
						:class="teamHealth.success_rate >= 0.99 ? 'status-ok' : teamHealth.success_rate >= 0.95 ? 'status-warning' : 'status-danger'"
					>
						{{ teamHealth.success_rate >= 0.99 ? '运行正常' : teamHealth.success_rate >= 0.95 ? '需关注' : '需处理' }}
					</span>
				</div>

				<div v-if="healthLoading" class="grid grid-cols-2 gap-3">
					<div v-for="index in 4" :key="index" class="skeleton h-[72px] rounded-2xl"></div>
				</div>
				<div v-else-if="teamHealth && teamHealth.total_requests > 0" class="grid grid-cols-2 gap-3">
					<div class="mini-tile">
						<p class="text-[11px] text-slate-400">调用成功率</p>
						<p class="mt-1 text-base font-bold tabular-nums text-emerald-600">{{ (teamHealth.success_rate * 100).toFixed(1) }}%</p>
					</div>
					<div class="mini-tile">
						<p class="text-[11px] text-slate-400">P95 响应</p>
						<p class="mt-1 text-base font-bold tabular-nums text-slate-700">{{ formatMs(teamHealth.p95_ms) }}</p>
					</div>
					<div class="mini-tile">
						<p class="text-[11px] text-slate-400">缓存命中率</p>
						<p class="mt-1 text-base font-bold tabular-nums text-cyan-600">{{ (teamHealth.cache_hit_ratio * 100).toFixed(1) }}%</p>
					</div>
					<div class="mini-tile">
						<p class="text-[11px] text-slate-400">平均首 Token</p>
						<p class="mt-1 text-base font-bold tabular-nums text-slate-700">{{ formatMs(teamHealth.avg_first_token_ms) }}</p>
					</div>
				</div>
				<p v-else class="py-8 text-center text-xs text-slate-400">本月暂无调用记录</p>
			</div>
		</section>

		<div v-if="!loading && !dashboardData" class="card">
			<div class="empty-state">
				<Icon name="chart" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无数据</p>
				<p class="empty-state-description">API 使用数据将在首次调用后展示</p>
			</div>
		</div>
	</div>
</template>

<style scoped>
.dashboard-shell {
	display: flex;
	flex-direction: column;
	gap: 1.25rem;
}

.refresh-button {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: 0.5rem;
	height: 2.5rem;
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

.refresh-button:hover:not(:disabled) {
	transform: translateY(-1px);
	background: white;
	color: #0d9488;
}

.refresh-button:disabled {
	cursor: wait;
	opacity: 0.65;
}

/* 顶部实时 RPM/TPM 速率胶囊（刷新按钮左侧），外观与刷新按钮呼应 */
.rate-pill {
	display: inline-flex;
	align-items: center;
	gap: 0.5rem;
	height: 2.5rem;
	padding: 0 1rem;
	border: 1px solid rgba(255, 255, 255, 0.88);
	border-radius: 9999px;
	background: rgba(255, 255, 255, 0.68);
	box-shadow: 0 8px 24px rgba(87, 102, 151, 0.08);
}

.rate-pill-label {
	font-size: 0.6875rem;
	font-weight: 600;
	letter-spacing: 0.05em;
	color: #94a3b8;
}

.rate-pill-value {
	font-size: 0.875rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	line-height: 1;
}

.rate-pill-sep {
	width: 1px;
	height: 0.875rem;
	background: #e2e8f0;
}

/* ── 指标卡 ──────────────────────────────────────────────
   设计取舍：
   1) 去掉顶部渐变装饰条——它不承载任何信息，四张卡并排时四条彩线互相抢；
   2) 去掉右上角徽标——「本月消费」配「本月」徽标是同义重复，位置让给数值；
   3) 颜色只用来表达状态：常态下仅图标底色带一点品牌色相，只有真正需要
      关注的卡（余额将耗尽、额度将用满）才亮起左侧状态条与状态色数值。
   这样四张卡平时是安静的，出问题的那张才会跳出来。 */
.metric-card {
	position: relative;
	display: flex;
	min-height: 152px;
	flex-direction: column;
	overflow: hidden;
	padding: 1.15rem 1.25rem 1.2rem;
	border: 1px solid var(--glass-border);
	border-radius: var(--radius-card);
	background: var(--glass-bg-strong);
	backdrop-filter: blur(var(--glass-blur)) saturate(var(--glass-saturate));
	box-shadow: var(--shadow-card);
	transition: transform 220ms ease, box-shadow 220ms ease;
}

@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
	.metric-card {
		background: rgba(255, 255, 255, 0.97);
	}
}

.metric-card:hover {
	transform: translateY(-2px);
	box-shadow: 0 18px 40px rgba(81, 94, 143, 0.13);
}

/* 状态条：四张卡都有，常态是极淡的中性色，只有进入告警态才变色。
   之前是"告警才出现"，结果一排卡里凭空多出一根线，读起来像没对齐的缺陷
   而不是提示——现在它是恒定的结构元素在变色，异常感消失，信号仍在。 */
.metric-card::before {
	position: absolute;
	top: 1.15rem;
	bottom: 1.2rem;
	left: 0;
	width: 3px;
	border-radius: 0 3px 3px 0;
	background: rgba(148, 163, 184, 0.22);
	content: '';
	transition: background 220ms ease;
}

.metric-warn::before {
	background: #f59e0b;
}

.metric-crit::before {
	background: #ef4444;
}

.metric-head {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: 0.75rem;
}

.metric-label {
	min-width: 0;
	overflow: hidden;
	color: #64748b;
	font-size: 0.8125rem;
	font-weight: 600;
	letter-spacing: 0.01em;
	text-overflow: ellipsis;
	white-space: nowrap;
}

/* 图标退到右上做定位锚点，不再与标签争夺左侧起始位 */
.metric-icon {
	display: flex;
	height: 1.85rem;
	width: 1.85rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 0.6rem;
	background: var(--metric-soft);
	color: var(--metric-color);
}

.metric-figure {
	display: flex;
	align-items: baseline;
	gap: 0.5rem;
	margin-top: 0.85rem;
	min-width: 0;
}

.metric-value {
	min-width: 0;
	overflow: hidden;
	color: #172033;
	font-size: 1.875rem;
	font-weight: 720;
	font-variant-numeric: tabular-nums;
	letter-spacing: -0.025em;
	line-height: 1.05;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.metric-crit .metric-value {
	color: #dc2626;
}

/* 环比是信息不是装饰，所以紧贴数值而不是塞进脚注 */
.metric-delta {
	flex-shrink: 0;
	border-radius: 0.35rem;
	padding: 0.1rem 0.35rem;
	font-size: 0.6875rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	white-space: nowrap;
}

.delta-up {
	background: #fef2f2;
	color: #dc2626;
}

.delta-down {
	background: #ecfdf5;
	color: #059669;
}

.delta-flat {
	background: #f1f5f9;
	color: #94a3b8;
}

/* 脚注贴底，保证四张卡在有无进度条时高度一致 */
.metric-sub {
	margin-top: auto;
	overflow: hidden;
	padding-top: 0.8rem;
	color: #94a3b8;
	font-size: 0.6875rem;
	line-height: 1.4;
	text-overflow: ellipsis;
	white-space: nowrap;
}

/* 进度条语义色，配合全局 .progress / .progress-bar 使用 */
.bar-ok {
	background: linear-gradient(90deg, #2dd4bf, #0d9488);
}

.bar-warn {
	background: linear-gradient(90deg, #fbbf24, #f59e0b);
}

.bar-crit {
	background: linear-gradient(90deg, #f87171, #ef4444);
}

.mini-tile {
	border: 1px solid rgba(255, 255, 255, 0.85);
	border-radius: 0.85rem;
	background: rgba(255, 255, 255, 0.6);
	padding: 0.7rem 0.8rem;
}

.member-avatar {
	display: flex;
	height: 2.25rem;
	width: 2.25rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 9999px;
	background: linear-gradient(145deg, hsl(var(--avatar-hue) 75% 72%), hsl(var(--avatar-hue) 68% 58%));
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.55);
	color: white;
	font-size: 0.75rem;
	font-weight: 700;
}

.quota-flag {
	flex-shrink: 0;
	border-radius: 0.35rem;
	background: #fffbeb;
	padding: 0.1rem 0.35rem;
	color: #d97706;
	font-size: 0.625rem;
	font-weight: 700;
	white-space: nowrap;
}

.trend-chip {
	flex-shrink: 0;
	border-radius: 0.35rem;
	padding: 0.15rem 0.4rem;
	font-size: 0.6875rem;
	font-weight: 700;
	font-variant-numeric: tabular-nums;
	white-space: nowrap;
}

.chip-up {
	background: #fef2f2;
	color: #dc2626;
}

.chip-down {
	background: #ecfdf5;
	color: #10b981;
}

.chip-flat {
	background: #f1f5f9;
	color: #94a3b8;
}

.status-dot {
	display: inline-flex;
	align-items: center;
	gap: 0.35rem;
	border-radius: 9999px;
	padding: 0.3rem 0.55rem;
	font-size: 0.6875rem;
	font-weight: 700;
}

.status-dot::before {
	height: 0.4rem;
	width: 0.4rem;
	border-radius: 9999px;
	background: currentColor;
	content: '';
}

.status-ok {
	background: #ecfdf5;
	color: #10b981;
}

.status-warning {
	background: #fffbeb;
	color: #f59e0b;
}

.status-danger {
	background: #fef2f2;
	color: #ef4444;
}

@keyframes wave {
	0%, 100% { transform: rotate(0deg); }
	25% { transform: rotate(14deg); }
	75% { transform: rotate(-8deg); }
}

.animate-wave {
	animation: wave 1.8s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
	.metric-card,
	.refresh-button,
	.animate-wave {
		animation: none;
		transition: none;
	}
}
</style>
