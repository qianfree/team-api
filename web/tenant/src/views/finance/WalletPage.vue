<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

const loading = ref(false)
const wallet = ref<any>(null)

// Recharge
const rechargeAmount = ref<number | null>(null)
const customAmount = ref('')
const rechargeLoading = ref(false)
const presetAmounts = [10, 50, 100, 500]

// Plans
const plans = ref<any[]>([])
const currentPlan = ref<any>(null)
const showConfirm = ref(false)
const selectedPlan = ref<any>(null)
const selectedMonths = ref(1)
const confirmLoading = ref(false)

// Frozen items
const showFrozenModal = ref(false)
const frozenItems = ref<any[]>([])
const frozenLoading = ref(false)
let frozenTimer: ReturnType<typeof setInterval> | null = null

const monthsOptions = [
	{ label: '1 个月', value: 1 },
	{ label: '3 个月', value: 3 },
	{ label: '6 个月', value: 6 },
	{ label: '12 个月（优惠）', value: 12 },
]

const currentPlanId = computed(() => currentPlan.value?.plan_id)

async function fetchWallet() {
	try {
		const res: any = await request.get('/tenant/wallet')
		wallet.value = res.data?.data
	} catch {
		wallet.value = null
	}
}

async function fetchPlans() {
	loading.value = true
	try {
		const [plansRes, currentRes] = await Promise.all([
			request.get('/tenant/plans'),
			request.get('/tenant/plan/current').catch(() => ({ data: { data: null } })),
		])
		const raw = plansRes.data?.data
		plans.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		currentPlan.value = currentRes.data?.data || null
	} catch {
		// interceptor handles error toast
	} finally {
		loading.value = false
	}
}

function selectPresetAmount(amount: number) {
	rechargeAmount.value = amount
	customAmount.value = ''
}

function onCustomInput() {
	const val = parseFloat(customAmount.value)
	rechargeAmount.value = isNaN(val) || val <= 0 ? null : val
}

async function handleRecharge() {
	if (!rechargeAmount.value || rechargeAmount.value <= 0) return
	rechargeLoading.value = true
	try {
		const res = await request.post('/tenant/orders/create', {
			order_type: 'recharge',
			amount: rechargeAmount.value,
		})
		const order = res.data?.data
		if (order?.id) {
			await request.post(`/tenant/orders/${order.id}/pay`)
		}
		rechargeAmount.value = null
		customAmount.value = ''
		fetchWallet()
	} catch {
		// interceptor handles error toast
	} finally {
		rechargeLoading.value = false
	}
}

function calcPrice(plan: any, months: number) {
	if (months >= 12) return Number(plan.yearly_price || 0)
	return Number(plan.monthly_price || 0) * months
}

function openConfirm(plan: any) {
	selectedPlan.value = plan
	selectedMonths.value = 1
	showConfirm.value = true
}

async function handleSubscribe() {
	confirmLoading.value = true
	try {
		const res = await request.post('/tenant/orders/create', {
			order_type: 'new_plan',
			plan_id: selectedPlan.value.id,
			months: selectedMonths.value,
		})
		const order = res.data?.data
		if (order?.id) {
			await request.post(`/tenant/orders/${order.id}/pay`)
		}
		showConfirm.value = false
		await fetchPlans()
	} catch {
		// interceptor handles error toast
	} finally {
		confirmLoading.value = false
	}
}

function isCurrentPlan(plan: any) {
	return currentPlan.value && currentPlan.value.plan_id === plan.id
}

// Frozen items
async function fetchFrozenItems() {
	frozenLoading.value = true
	try {
		const res: any = await request.get('/tenant/wallet/frozen-items')
		frozenItems.value = res.data?.data?.items || []
	} catch {
		frozenItems.value = []
	} finally {
		frozenLoading.value = false
	}
}

function openFrozenModal() {
	showFrozenModal.value = true
	fetchFrozenItems()
	frozenTimer = setInterval(fetchFrozenItems, 10000)
}

function closeFrozenModal() {
	showFrozenModal.value = false
	if (frozenTimer) {
		clearInterval(frozenTimer)
		frozenTimer = null
	}
}

function formatTime(unix: number): string {
	if (!unix) return '-'
	return new Date(unix * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatRemaining(seconds: number): string {
	if (seconds <= 0) return '即将到期'
	const m = Math.floor(seconds / 60)
	const s = seconds % 60
	return m > 0 ? `${m}分${s}秒` : `${s}秒`
}

onMounted(() => {
	fetchWallet()
	fetchPlans()
})

onBeforeUnmount(() => {
	if (frozenTimer) {
		clearInterval(frozenTimer)
	}
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<h1 class="page-title">钱包</h1>
			<p class="page-description">管理余额、充值和套餐方案</p>
		</div>

		<!-- Wallet Summary -->
		<div class="grid grid-cols-1 sm:grid-cols-3 gap-5">
			<div class="card p-5">
				<p class="stat-label">可用余额</p>
				<p class="text-2xl font-bold text-gray-900">${{ wallet?.available_balance?.toFixed(2) ?? '0.00' }}</p>
				<p class="text-xs text-gray-400 mt-1">{{ wallet?.currency || 'USD' }}</p>
			</div>
			<div
				class="card p-5 cursor-pointer hover:-translate-y-0.5 transition-all duration-200 group"
				@click="openFrozenModal"
			>
				<p class="stat-label">冻结余额</p>
				<p class="text-2xl font-bold text-amber-600 group-hover:text-amber-700 transition-colors">
					${{ wallet?.frozen_balance?.toFixed(2) ?? '0.00' }}
				</p>
				<p v-if="wallet?.frozen_balance > 0" class="text-xs text-amber-500 mt-1">点击查看明细</p>
			</div>
			<div class="card p-5">
				<p class="stat-label">总余额</p>
				<p class="text-2xl font-bold text-gray-900">${{ wallet?.balance?.toFixed(2) ?? '0.00' }}</p>
			</div>
		</div>

		<!-- Recharge -->
		<div class="card">
			<div class="card-header">
				<h2 class="font-semibold text-gray-900">充值</h2>
			</div>
			<div class="card-body">
				<div class="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
					<button
						v-for="amt in presetAmounts"
						:key="amt"
						class="rounded-xl px-4 py-3 text-center border-2 font-semibold text-gray-900 transition-all hover:shadow-sm"
						:class="rechargeAmount === amt
							? 'border-primary-500 bg-primary-50 shadow-glow'
							: 'border-gray-200 hover:border-gray-300'"
						@click="selectPresetAmount(amt)"
					>
						¥{{ amt }}
					</button>
				</div>
				<div class="flex items-center gap-3">
					<input
						v-model="customAmount"
						type="number"
						class="input flex-1"
						placeholder="自定义金额"
						min="1"
						step="0.01"
						@input="onCustomInput"
					/>
					<button
						class="btn btn-primary"
						:disabled="!rechargeAmount || rechargeLoading"
						@click="handleRecharge"
					>
						{{ rechargeLoading ? '处理中...' : '立即充值' }}
					</button>
				</div>
			</div>
		</div>

		<!-- Subscription Plans -->
		<div class="card">
			<div class="card-header">
				<h2 class="font-semibold text-gray-900">套餐方案</h2>
				<p class="text-sm text-gray-500 mt-0.5">选择适合您团队的方案</p>
			</div>
			<div class="card-body">
				<!-- Loading -->
				<div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					<div v-for="i in 3" :key="i" class="p-4 border border-gray-200 rounded-xl animate-pulse">
						<div class="h-5 bg-gray-200 rounded w-1/2 mb-3"></div>
						<div class="h-7 bg-gray-200 rounded w-3/4 mb-4"></div>
						<div class="h-4 bg-gray-200 rounded w-full"></div>
					</div>
				</div>

				<!-- Empty -->
				<div v-else-if="plans.length === 0" class="py-8 text-center">
					<Icon name="exclamationCircle" size="xl" class="text-gray-300 mx-auto mb-3" />
					<p class="text-gray-500 text-sm">暂无可用套餐，请联系管理员配置</p>
				</div>

				<!-- Plan Cards -->
				<div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
					<div
						v-for="plan in plans"
						:key="plan.id"
						class="relative flex flex-col p-4 border rounded-xl transition-all hover:shadow-sm"
						:class="plan.is_recommended
							? 'border-primary-500 bg-primary-50/30 ring-1 ring-primary-500/20'
							: 'border-gray-200 hover:border-gray-300'"
					>
						<!-- Recommended Badge -->
						<div
							v-if="plan.is_recommended"
							class="absolute -top-2.5 left-1/2 -translate-x-1/2"
						>
							<span class="badge badge-primary text-xs">推荐</span>
						</div>

						<!-- Current Plan Badge -->
						<div
							v-if="isCurrentPlan(plan)"
							class="absolute top-3 right-3"
						>
							<span class="badge badge-success text-xs">当前</span>
						</div>

						<!-- Plan Info -->
						<h3 class="text-base font-semibold text-gray-900 mb-1">{{ plan.name }}</h3>
						<p class="text-xs text-gray-500 mb-3">{{ plan.description || '' }}</p>

						<!-- Price -->
						<div class="mb-4">
							<div class="flex items-baseline gap-1">
								<span class="text-2xl font-bold text-gray-900">¥{{ Number(plan.monthly_price).toFixed(0) }}</span>
								<span class="text-xs text-gray-500">/月</span>
							</div>
							<p v-if="Number(plan.yearly_price) > 0" class="text-xs text-gray-400 mt-1">
								年付 ¥{{ Number(plan.yearly_price).toFixed(0) }}（省 ¥{{ (Number(plan.monthly_price) * 12 - Number(plan.yearly_price)).toFixed(0) }}）
							</p>
						</div>

						<!-- Features -->
						<div class="space-y-1.5 text-sm text-gray-600 flex-1">
							<div v-if="plan.monthly_quota_tokens > 0" class="flex items-center gap-1.5">
								<Icon name="check" size="sm" class="text-primary-500 flex-shrink-0" />
								<span class="text-xs">{{ plan.monthly_quota_tokens.toLocaleString() }} Tokens/月</span>
							</div>
							<div v-else class="flex items-center gap-1.5">
								<Icon name="check" size="sm" class="text-primary-500 flex-shrink-0" />
								<span class="text-xs">不限 Token 用量</span>
							</div>
						</div>

						<!-- Action -->
						<div class="mt-4 pt-3 border-t border-gray-100">
							<button
								v-if="isCurrentPlan(plan)"
								class="btn btn-secondary w-full btn-sm"
								disabled
							>
								当前方案
							</button>
							<button
								v-else
								class="btn btn-primary w-full btn-sm"
								@click="openConfirm(plan)"
							>
								{{ Number(plan.monthly_price) === 0 ? '免费开通' : '立即购买' }}
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- Confirm Modal -->
		<Teleport to="body">
			<transition name="modal">
				<div v-if="showConfirm" class="modal-overlay" @click.self="showConfirm = false">
					<div class="modal-content w-full max-w-md">
						<div class="modal-header">
							<h3 class="modal-title">确认订阅</h3>
							<button @click="showConfirm = false" class="btn-ghost btn-icon">
								<Icon name="x" size="md" />
							</button>
						</div>
						<div class="modal-body">
							<div v-if="selectedPlan" class="space-y-4">
								<div class="flex items-center justify-between p-3 bg-gray-50 rounded-xl">
									<span class="font-medium text-gray-900">{{ selectedPlan.name }}</span>
									<span class="badge badge-primary">{{ selectedPlan.identifier }}</span>
								</div>

								<div>
									<label class="input-label">订阅时长</label>
									<div class="grid grid-cols-2 gap-2 mt-1.5">
										<button
											v-for="opt in monthsOptions"
											:key="opt.value"
											class="rounded-xl px-3 py-2 text-sm font-medium border transition-all"
											:class="selectedMonths === opt.value
												? 'border-primary-500 bg-primary-50 text-primary-700'
												: 'border-gray-200 text-gray-600 hover:border-gray-300'"
											@click="selectedMonths = opt.value"
										>
											{{ opt.label }}
										</button>
									</div>
								</div>

								<div class="flex items-center justify-between p-3 bg-primary-50 rounded-xl">
									<span class="text-sm text-gray-600">应付金额</span>
									<span class="text-xl font-bold text-primary-600">
										¥{{ calcPrice(selectedPlan, selectedMonths).toFixed(2) }}
									</span>
								</div>
							</div>
						</div>
						<div class="modal-footer">
							<button @click="showConfirm = false" class="btn btn-secondary">取消</button>
							<button
								class="btn btn-primary"
								:disabled="confirmLoading"
								@click="handleSubscribe"
							>
								{{ confirmLoading ? '处理中...' : '确认订阅' }}
							</button>
						</div>
					</div>
				</div>
			</transition>
		</Teleport>

		<!-- Frozen Items Modal -->
		<Teleport to="body">
			<transition name="modal">
				<div v-if="showFrozenModal" class="modal-overlay" @click.self="closeFrozenModal">
					<div class="modal-content w-full max-w-lg">
						<div class="modal-header">
							<h3 class="modal-title">冻结明细</h3>
							<button @click="closeFrozenModal" class="btn-ghost btn-icon">
								<Icon name="x" size="md" />
							</button>
						</div>
						<div class="modal-body">
							<!-- Loading -->
							<div v-if="frozenLoading && frozenItems.length === 0" class="space-y-3">
								<div v-for="i in 3" :key="i" class="h-12 bg-gray-100 rounded-xl animate-pulse"></div>
							</div>
							<!-- Empty -->
							<div v-else-if="frozenItems.length === 0" class="py-8 text-center">
								<Icon name="exclamationCircle" size="xl" class="text-gray-300 mx-auto mb-3" />
								<p class="text-gray-500 text-sm">当前没有冻结的资金</p>
							</div>
							<!-- Items list -->
							<div v-else class="space-y-2">
								<div v-for="item in frozenItems" :key="item.request_id"
									class="flex items-center justify-between p-3 bg-gray-50 rounded-xl">
									<div class="min-w-0 flex-1">
										<p class="text-sm font-medium text-gray-900 truncate">{{ item.model_name || '未知模型' }}</p>
										<p class="text-xs text-gray-400 truncate mt-0.5">
											{{ item.request_id.substring(0, 16) }}...
											<span class="ml-2">{{ formatTime(item.created_at) }}</span>
										</p>
									</div>
									<div class="text-right ml-3 flex-shrink-0">
										<p class="text-sm font-semibold text-amber-600">${{ item.amount?.toFixed(4) }}</p>
										<p class="text-xs text-gray-400">剩余 {{ formatRemaining(item.remaining) }}</p>
									</div>
								</div>
							</div>
						</div>
						<div class="modal-footer">
							<p class="text-xs text-gray-400 flex-1">自动刷新中 · 每 10 秒更新</p>
							<button @click="closeFrozenModal" class="btn btn-secondary btn-sm">关闭</button>
						</div>
					</div>
				</div>
			</transition>
		</Teleport>
	</div>
</template>
