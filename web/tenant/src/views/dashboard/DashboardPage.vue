<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

interface DashboardData {
	today: DayStats | null
	month: DayStats | null
	wallet: WalletInfo | null
	rpm: number
	tpm: number
	active_keys: number
	member_count: number
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
}

interface PredictionData {
	daily_avg_cost: number
	available_balance: number
	will_exhaust: boolean
	days_until_exhaust?: number
	exhaust_date?: string
	message?: string
}

interface BudgetAlert {
	members: Array<{
		id: number
		username: string
		display_name: string
		quota_limit: number
		used_cost: number
		usage_percent: number
	}>
	projects: Array<{
		id: number
		name: string
		budget_limit: number
		used_cost: number
		usage_percent: number
	}>
}

const authStore = useTenantAuthStore()
const loading = ref(false)
const chartsLoading = ref(false)
const memberUsageLoading = ref(false)
const alertsLoading = ref(false)
const selectedDays = ref(7)
const dashboardData = ref<DashboardData | null>(null)
const trendData = ref<TrendPoint[]>([])
const modelData = ref<ModelItem[]>([])
const memberUsageData = ref<MemberUsageItem[]>([])
const predictionData = ref<PredictionData | null>(null)
const alertsData = ref<BudgetAlert | null>(null)

const safeDay = (day: DayStats | null | undefined): DayStats =>
	day || { requests: 0, input_tokens: 0, output_tokens: 0, total_cost: 0 }

const safeWallet = (wallet: WalletInfo | null | undefined): WalletInfo =>
	wallet || { balance: 0, frozen_balance: 0, available: 0, warning_threshold: 0 }

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

const coreStats = computed(() => {
	if (!dashboardData.value) return []
	const today = safeDay(dashboardData.value.today)
	const month = safeDay(dashboardData.value.month)
	const monthTokens = month.input_tokens + month.output_tokens
	return [
		{
			label: '今日请求',
			value: formatNumber(today.requests),
			sub: `输入 ${formatNumber(today.input_tokens)} · 输出 ${formatNumber(today.output_tokens)}`,
			trend: '实时统计',
				icon: 'play',
				color: '#7667f6',
				soft: 'rgba(118, 103, 246, 0.14)',
		},
		{
			label: '本月请求',
			value: formatNumber(month.requests),
			sub: `日均 ${formatNumber(month.requests / Math.max(new Date().getDate(), 1))} 次`,
			trend: '本月累计',
				icon: 'chart',
				color: '#3b9df8',
				soft: 'rgba(59, 157, 248, 0.14)',
		},
		{
			label: '本月 Token',
			value: formatNumber(monthTokens),
			sub: `输入 ${formatNumber(month.input_tokens)} · 输出 ${formatNumber(month.output_tokens)}`,
			trend: '调用消耗',
				icon: 'bolt',
				color: '#22c7b7',
				soft: 'rgba(34, 199, 183, 0.14)',
		},
		{
			label: '本月消费',
			value: formatCost(month.total_cost),
			sub: `今日 ${formatCost(today.total_cost)}`,
			trend: '费用明细',
				icon: 'creditCard',
				color: '#9a58ee',
				soft: 'rgba(154, 88, 238, 0.14)',
		},
	]
})

const accountStats = computed(() => {
	if (!dashboardData.value) return []
	const wallet = safeWallet(dashboardData.value.wallet)
	const prediction = predictionData.value
	let predictionValue = '安全'
	let predictionTone = 'text-emerald-600'
	let predictionDescription = prediction?.message || '余额状态稳定'

	if (prediction?.will_exhaust && prediction.days_until_exhaust !== undefined) {
		predictionValue = `${prediction.days_until_exhaust} 天`
		predictionTone = prediction.days_until_exhaust <= 7 ? 'text-red-500' : 'text-amber-500'
		predictionDescription = `预计 ${prediction.exhaust_date || '近期'} 耗尽`
	}

	return [
		{ label: '可用余额', value: formatCost(wallet.available), description: `冻结 ${formatCost(wallet.frozen_balance)}`, icon: 'wallet', tone: 'text-violet-600', background: 'bg-violet-50' },
		{ label: '活跃密钥', value: String(dashboardData.value.active_keys || 0), description: '个 API Key', icon: 'key', tone: 'text-blue-600', background: 'bg-blue-50' },
		{ label: '团队成员', value: String(dashboardData.value.member_count || 0), description: '位活跃成员', icon: 'users', tone: 'text-cyan-600', background: 'bg-cyan-50' },
		{ label: '余额预测', value: predictionValue, description: predictionDescription, icon: 'trendingUp', tone: predictionTone, background: 'bg-emerald-50' },
	]
})

const alertItems = computed(() => {
	const members = (alertsData.value?.members || []).map((item) => ({
		id: `member-${item.id}`,
		name: item.display_name || item.username,
		percent: item.usage_percent,
		type: '成员额度',
	}))
	const projects = (alertsData.value?.projects || []).map((item) => ({
		id: `project-${item.id}`,
		name: item.name,
		percent: item.usage_percent,
		type: '项目预算',
	}))
	return [...members, ...projects].slice(0, 3)
})

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
		alertsData.value = response.data?.data || null
	} catch {
		alertsData.value = null
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
	} catch {
		memberUsageData.value = []
	} finally {
		memberUsageLoading.value = false
	}
}

function refreshAll() {
	void fetchDashboard()
	void fetchCharts()
	void fetchPrediction()
	void fetchAlerts()
	void fetchMemberUsage()
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
				<p class="mt-1 text-sm text-slate-400">今天是 {{ formattedDate }}，查看团队 API 的最新运行状态。</p>
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
				<div v-for="index in 4" :key="index" class="stat-card h-[152px]">
					<div class="flex items-center justify-between">
						<div class="skeleton h-10 w-10 rounded-xl"></div>
						<div class="skeleton h-5 w-16 rounded-full"></div>
					</div>
					<div class="skeleton mt-4 h-8 w-28"></div>
					<div class="skeleton mt-3 h-3.5 w-40"></div>
			</div>
		</section>

		<section v-else-if="dashboardData" class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
			<article
				v-for="stat in coreStats"
				:key="stat.label"
				class="stat-card metric-card group"
				:style="{ '--metric-color': stat.color, '--metric-soft': stat.soft }"
			>
					<div class="metric-accent" aria-hidden="true"></div>
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-3">
							<div class="metric-icon">
								<Icon :name="stat.icon" size="md" />
							</div>
							<p class="truncate text-sm font-semibold text-slate-600">{{ stat.label }}</p>
						</div>
						<span class="metric-badge">{{ stat.trend }}</span>
					</div>
					<p class="metric-value" :title="stat.value">{{ stat.value }}</p>
					<div class="metric-detail">
						<span class="metric-detail-dot" aria-hidden="true"></span>
						<span class="truncate">{{ stat.sub }}</span>
					</div>
			</article>
		</section>

		<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
			<TokenTrendChart :data="trendData" :loading="chartsLoading" :days="selectedDays" @change-days="selectedDays = $event" />
			<div class="card card-prominent p-5 sm:p-6">
				<div class="mb-4 flex items-center justify-between">
					<div>
						<h2 class="text-base font-semibold text-slate-900">账户概览</h2>
						<p class="mt-0.5 text-xs text-slate-400">余额、成员与密钥状态</p>
					</div>
					<span v-if="!alertsLoading" class="status-dot" :class="alertItems.length ? 'status-warning' : 'status-ok'">{{ alertItems.length ? '需关注' : '运行正常' }}</span>
				</div>
				<div class="grid grid-cols-2 gap-3">
					<div v-for="item in accountStats" :key="item.label" class="rounded-2xl border border-white/80 bg-white/55 p-3.5">
						<div class="mb-3 flex h-8 w-8 items-center justify-center rounded-xl" :class="[item.background, item.tone]">
							<Icon :name="item.icon" size="sm" />
						</div>
						<p class="text-xs text-slate-400">{{ item.label }}</p>
						<p class="mt-1 truncate text-base font-bold" :class="item.tone">{{ item.value }}</p>
						<p class="mt-1 truncate text-[11px] text-slate-400">{{ item.description }}</p>
					</div>
				</div>
				<div v-if="alertItems.length" class="mt-4 space-y-2 border-t border-slate-100 pt-4">
					<div v-for="alert in alertItems" :key="alert.id" class="flex items-center gap-3 rounded-xl bg-amber-50/80 px-3 py-2.5">
						<Icon name="exclamationTriangle" size="sm" class="flex-shrink-0 text-amber-500" />
						<div class="min-w-0 flex-1">
							<p class="truncate text-xs font-medium text-slate-700">{{ alert.name }}</p>
							<p class="text-[10px] text-slate-400">{{ alert.type }}</p>
						</div>
						<span class="text-xs font-bold text-amber-600">{{ alert.percent }}%</span>
					</div>
				</div>
			</div>
		</section>

		<section class="grid grid-cols-1 gap-5 2xl:grid-cols-[minmax(0,1.45fr)_minmax(360px,.9fr)]">
			<div class="card card-prominent overflow-hidden">
				<div class="flex items-center justify-between border-b border-slate-100/80 px-5 py-4 sm:px-6">
					<div>
						<h2 class="text-base font-semibold text-slate-900">成员用量排行</h2>
						<p class="mt-0.5 text-xs text-slate-400">近 {{ selectedDays }} 天团队调用费用</p>
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
					<div v-for="(member, index) in memberUsageData" :key="member.user_id" class="grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 py-3.5">
						<span class="flex h-7 w-7 items-center justify-center rounded-lg text-xs font-bold" :class="index < 3 ? 'bg-primary-50 text-primary-600' : 'bg-slate-50 text-slate-400'">{{ index + 1 }}</span>
						<div class="flex min-w-0 items-center gap-3">
							<div class="member-avatar" :style="{ '--avatar-hue': `${(member.user_id * 43) % 360}` }">{{ (member.display_name || member.username).charAt(0).toUpperCase() }}</div>
							<div class="min-w-0">
								<p class="truncate text-sm font-medium text-slate-700">{{ member.display_name || member.username }}</p>
								<p class="truncate text-xs text-slate-400">{{ formatNumber(member.requests) }} 次请求 · {{ formatNumber(member.input_tokens + member.output_tokens) }} Token</p>
							</div>
						</div>
						<span class="text-sm font-semibold tabular-nums text-slate-700">{{ formatCost(member.total_cost) }}</span>
					</div>
				</div>
			</div>

			<ModelDistChart :data="modelData" :loading="chartsLoading" />
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

.metric-card {
	min-height: 152px;
	padding: 1.125rem 1.25rem;
	transition: transform 220ms ease, box-shadow 220ms ease;
}

.metric-card:hover {
	transform: translateY(-3px);
	box-shadow: 0 20px 45px rgba(81, 94, 143, 0.14);
}

.metric-accent {
	position: absolute;
	top: 0;
	right: 1.25rem;
	left: 1.25rem;
	height: 2px;
	border-radius: 0 0 9999px 9999px;
	background: linear-gradient(90deg, transparent, var(--metric-color), transparent);
	opacity: 0.7;
}

.metric-icon {
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

.metric-badge {
	flex-shrink: 0;
	border: 1px solid color-mix(in srgb, var(--metric-color) 20%, transparent);
	border-radius: 9999px;
	background: var(--metric-soft);
	padding: 0.25rem 0.5rem;
	color: var(--metric-color);
	font-size: 0.625rem;
	font-weight: 700;
}

.metric-value {
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

.metric-detail {
	display: flex;
	min-width: 0;
	align-items: center;
	gap: 0.5rem;
	margin-top: 0.75rem;
	color: #94a3b8;
	font-size: 0.6875rem;
}

.metric-detail-dot {
	height: 0.375rem;
	width: 0.375rem;
	flex-shrink: 0;
	border-radius: 9999px;
	background: var(--metric-color);
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
