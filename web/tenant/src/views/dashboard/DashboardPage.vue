<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import TokenTrendChart from '@/components/charts/TokenTrendChart.vue'
import ModelDistChart from '@/components/charts/ModelDistChart.vue'
import request from '@/utils/request'

// ===== State =====
const loading = ref(false)
const chartsLoading = ref(false)
const memberUsageLoading = ref(false)
const alertsLoading = ref(false)

const selectedDays = ref(7)

// ===== Data =====
interface DayStats { requests: number; input_tokens: number; output_tokens: number; total_cost: number }
interface WalletInfo { balance: number; frozen_balance: number; available: number; warning_threshold: number }
interface DashboardData {
	today: DayStats | null
	month: DayStats | null
	wallet: WalletInfo | null
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

const dashboardData = ref<DashboardData | null>(null)
const trendData = ref<TrendPoint[]>([])
const modelData = ref<ModelItem[]>([])
const memberUsageData = ref<MemberUsageItem[]>([])
const predictionData = ref<PredictionData | null>(null)
const alertsData = ref<BudgetAlert | null>(null)

// ===== Safe Helpers =====
const safeDay = (d: DayStats | null | undefined): DayStats =>
	d || { requests: 0, input_tokens: 0, output_tokens: 0, total_cost: 0 }
const safeWallet = (w: WalletInfo | null | undefined): WalletInfo =>
	w || { balance: 0, frozen_balance: 0, available: 0, warning_threshold: 0 }

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

// ===== Core Stats Cards =====
const coreStats = computed(() => {
	if (!dashboardData.value) return []
	const today = safeDay(dashboardData.value.today)
	const month = safeDay(dashboardData.value.month)
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

// ===== Account Stats Cards =====
const accountStats = computed(() => {
	if (!dashboardData.value) return []
	const d = dashboardData.value
	const w = safeWallet(d.wallet)
	const pred = predictionData.value

	const balanceCard = {
		label: '可用余额',
		value: formatCost(w.available),
		sub: `冻结 ${formatCost(w.frozen_balance)}`,
		icon: 'wallet',
		iconClass: 'bg-primary-100 text-primary-600',
	}

	const keyCard = {
		label: '活跃 Key',
		value: String(d.active_keys || 0),
		sub: '个 API 密钥',
		icon: 'key',
		iconClass: 'bg-violet-100 text-violet-600',
	}

	const memberCard = {
		label: '团队成员',
		value: String(d.member_count || 0),
		sub: '位活跃成员',
		icon: 'users',
		iconClass: 'bg-cyan-100 text-cyan-600',
	}

	let predValue = '—'
	let predSub = '计算中...'
	let predClass = 'bg-gray-100 text-gray-600'
	if (pred) {
		if (!pred.will_exhaust) {
			predValue = '安全'
			predSub = pred.message || '近期无消耗'
			predClass = 'bg-emerald-100 text-emerald-600'
		} else if (pred.days_until_exhaust !== undefined) {
			predValue = `${pred.days_until_exhaust} 天`
			predSub = `预计 ${pred.exhaust_date || '—'} 耗尽`
			predClass = pred.days_until_exhaust <= 7 ? 'bg-red-100 text-red-600' : 'bg-amber-100 text-amber-600'
		}
	}
	const predCard = {
		label: '余额预测',
		value: predValue,
		sub: predSub,
		icon: 'trendingUp',
		iconClass: predClass,
	}

	return [balanceCard, keyCard, memberCard, predCard]
})

// ===== Budget Alerts =====
const hasAlerts = computed(() => {
	if (!alertsData.value) return false
	return (alertsData.value.members?.length || 0) > 0 || (alertsData.value.projects?.length || 0) > 0
})

// ===== Data Loading =====
function ensureArray<T>(val: any): T[] {
	if (Array.isArray(val)) return val
	return []
}

async function fetchDashboard() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/dashboard')
		dashboardData.value = res.data?.data || null
	} catch {
		dashboardData.value = null
	} finally {
		loading.value = false
	}
}

async function fetchCharts() {
	chartsLoading.value = true
	try {
		const [trendRes, modelRes]: any[] = await Promise.all([
			request.get('/tenant/dashboard/token-trends', { params: { days: selectedDays.value } }),
			request.get('/tenant/dashboard/model-distribution', { params: { days: selectedDays.value } }),
		])
		trendData.value = ensureArray(trendRes.data?.data?.list)
		modelData.value = ensureArray(modelRes.data?.data?.list)
	} catch {
		trendData.value = []
		modelData.value = []
	} finally {
		chartsLoading.value = false
	}
}

async function fetchPrediction() {
	try {
		const res: any = await request.get('/tenant/dashboard/balance-prediction')
		predictionData.value = res.data?.data || null
	} catch {
		predictionData.value = null
	}
}

async function fetchAlerts() {
	alertsLoading.value = true
	try {
		const res: any = await request.get('/tenant/dashboard/budget-alerts')
		alertsData.value = res.data?.data || null
	} catch {
		alertsData.value = null
	} finally {
		alertsLoading.value = false
	}
}

async function fetchMemberUsage() {
	memberUsageLoading.value = true
	try {
		const res: any = await request.get('/tenant/dashboard/member-usage-ranking', {
			params: { days: selectedDays.value, limit: 10 }
		})
		memberUsageData.value = ensureArray(res.data?.data?.list)
	} catch {
		memberUsageData.value = []
	} finally {
		memberUsageLoading.value = false
	}
}

function refreshAll() {
	fetchDashboard()
	fetchCharts()
	fetchPrediction()
	fetchAlerts()
	fetchMemberUsage()
}

watch(selectedDays, () => {
	fetchCharts()
	fetchMemberUsage()
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
					<h1 class="page-title">控制台</h1>
					<p class="page-description">API 使用量和消费概览</p>
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
		<div v-else-if="dashboardData" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
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

		<!-- ===== Row 2: Account Stats ===== -->
		<div v-if="loading" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
			<div v-for="i in 4" :key="'acc'+i" class="stat-card">
				<div class="skeleton h-12 w-12 rounded-xl"></div>
				<div class="flex-1">
					<div class="skeleton h-4 w-16 mb-2"></div>
					<div class="skeleton h-7 w-24"></div>
				</div>
			</div>
		</div>
		<div v-else-if="dashboardData" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
			<div v-for="stat in accountStats" :key="stat.label" class="stat-card">
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

		<!-- ===== Row 3: Charts ===== -->
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

		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<TokenTrendChart :data="trendData" :loading="chartsLoading" />
			<ModelDistChart :data="modelData" :loading="chartsLoading" />
		</div>

		<!-- ===== Row 4: Member Usage + Budget Alerts ===== -->
		<div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
			<!-- Member Usage Ranking -->
			<div class="card p-6">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-base font-semibold text-gray-900">成员用量排名</h3>
					<span class="text-xs text-gray-400">近 {{ selectedDays }} 天</span>
				</div>

				<div v-if="memberUsageLoading" class="space-y-3">
					<div v-for="i in 3" :key="i" class="flex items-center gap-3">
						<div class="skeleton h-8 w-8 rounded-full"></div>
						<div class="flex-1">
							<div class="skeleton h-4 w-24 mb-1"></div>
							<div class="skeleton h-3 w-32"></div>
						</div>
						<div class="skeleton h-4 w-16"></div>
					</div>
				</div>

				<div v-else-if="memberUsageData.length === 0" class="py-8 text-center text-gray-400 text-sm">
					暂无成员用量数据
				</div>

				<div v-else class="space-y-1">
					<div
						v-for="(member, idx) in memberUsageData"
						:key="member.user_id"
						class="flex items-center gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 transition-colors"
					>
						<span
							class="w-6 h-6 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0"
							:class="idx < 3 ? 'bg-primary-100 text-primary-600' : 'bg-gray-100 text-gray-500'"
						>
							{{ idx + 1 }}
						</span>
						<div class="w-8 h-8 rounded-full bg-gradient-to-br from-primary-400 to-primary-600 flex items-center justify-center text-white text-xs font-medium flex-shrink-0">
							{{ (member.display_name || member.username || '?').charAt(0).toUpperCase() }}
						</div>
						<div class="flex-1 min-w-0">
							<p class="text-sm font-medium text-gray-900 truncate">{{ member.display_name || member.username }}</p>
							<p class="text-xs text-gray-400 truncate">{{ formatNumber(member.requests) }} 次请求 / {{ formatNumber((member.input_tokens || 0) + (member.output_tokens || 0)) }} Token</p>
						</div>
						<span class="text-sm font-semibold text-emerald-600 tabular-nums flex-shrink-0">
							{{ formatCost(member.total_cost) }}
						</span>
					</div>
				</div>
			</div>

			<!-- Budget Alerts -->
			<div class="card p-6">
				<div class="flex items-center justify-between mb-4">
					<h3 class="text-base font-semibold text-gray-900">预算告警</h3>
					<Icon v-if="hasAlerts" name="exclamationTriangle" size="md" class="text-amber-500" />
					<Icon v-else name="checkCircle" size="md" class="text-emerald-500" />
				</div>

				<div v-if="alertsLoading" class="space-y-3">
					<div v-for="i in 2" :key="i" class="skeleton h-16 rounded-xl"></div>
				</div>

				<div v-else-if="!hasAlerts" class="py-8 text-center">
					<div class="inline-flex items-center justify-center w-12 h-12 rounded-full bg-emerald-100 mb-3">
						<Icon name="checkCircle" size="lg" class="text-emerald-500" />
					</div>
					<p class="text-sm text-gray-500">所有预算使用正常</p>
					<p class="text-xs text-gray-400 mt-1">没有超过 80% 的预算项</p>
				</div>

				<div v-else class="space-y-3">
					<template v-if="alertsData?.members?.length">
						<p class="text-xs font-medium text-gray-500 uppercase tracking-wider">成员额度</p>
						<div
							v-for="m in alertsData.members"
							:key="'m-'+m.id"
							class="flex items-center gap-3 p-3 rounded-xl bg-amber-50 border border-amber-100"
						>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between mb-1">
									<span class="text-sm font-medium text-gray-900 truncate">{{ m.display_name || m.username }}</span>
									<span class="text-xs font-semibold" :class="m.usage_percent >= 100 ? 'text-red-600' : 'text-amber-600'">
										{{ m.usage_percent }}%
									</span>
								</div>
								<div class="progress">
									<div
										class="progress-bar"
										:style="{ width: Math.min(m.usage_percent, 100) + '%' }"
										:class="m.usage_percent >= 100 ? '!bg-gradient-to-r !from-red-500 !to-red-400' : '!bg-gradient-to-r !from-amber-500 !to-amber-400'"
									></div>
								</div>
								<div class="flex justify-between mt-1">
									<span class="text-xs text-gray-500">已用 {{ formatCost(m.used_cost) }}</span>
									<span class="text-xs text-gray-500">额度 {{ formatCost(m.quota_limit) }}</span>
								</div>
							</div>
						</div>
					</template>

					<template v-if="alertsData?.projects?.length">
						<p class="text-xs font-medium text-gray-500 uppercase tracking-wider mt-4">项目预算</p>
						<div
							v-for="p in alertsData.projects"
							:key="'p-'+p.id"
							class="flex items-center gap-3 p-3 rounded-xl bg-amber-50 border border-amber-100"
						>
							<div class="flex-1 min-w-0">
								<div class="flex items-center justify-between mb-1">
									<span class="text-sm font-medium text-gray-900 truncate">{{ p.name }}</span>
									<span class="text-xs font-semibold" :class="p.usage_percent >= 100 ? 'text-red-600' : 'text-amber-600'">
										{{ p.usage_percent }}%
									</span>
								</div>
								<div class="progress">
									<div
										class="progress-bar"
										:style="{ width: Math.min(p.usage_percent, 100) + '%' }"
										:class="p.usage_percent >= 100 ? '!bg-gradient-to-r !from-red-500 !to-red-400' : '!bg-gradient-to-r !from-amber-500 !to-amber-400'"
									></div>
								</div>
								<div class="flex justify-between mt-1">
									<span class="text-xs text-gray-500">已用 {{ formatCost(p.used_cost) }}</span>
									<span class="text-xs text-gray-500">预算 {{ formatCost(p.budget_limit) }}</span>
								</div>
							</div>
						</div>
					</template>
				</div>
			</div>
		</div>

		<!-- ===== Empty State ===== -->
		<div v-if="!loading && !dashboardData" class="card">
			<div class="empty-state">
				<Icon name="chart" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无数据</p>
				<p class="empty-state-description">API 使用数据将在您首次调用后展示</p>
			</div>
		</div>
	</div>
</template>
