<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

const route = useRoute()
const loading = ref(false)
const wallet = ref<any>(null)

// Recharge
const rechargeAmount = ref<number | null>(null)
const customAmount = ref('')
const rechargeLoading = ref(false)
const selectedChannel = ref('')
const selectedPaymentMethod = ref('')
const paymentInfo = ref<any>(null)

// Pay result notification
const payResult = ref<'success' | 'fail' | ''>('')

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

// Computed: preset amounts from payment settings
const presetAmounts = computed(() => {
	if (paymentInfo.value?.amount_options?.length) {
		return paymentInfo.value.amount_options
	}
	return [10, 50, 100, 500]
})

// Computed: flattened pay methods from all channels
const payMethods = computed(() => {
	if (!paymentInfo.value?.channels) return []
	const methods: { channel: string; type: string; name: string; color: string }[] = []
	for (const ch of paymentInfo.value.channels) {
		if (ch.pay_methods?.length) {
			for (const m of ch.pay_methods) {
				methods.push({
					channel: ch.channel,
					type: m.type,
					name: m.name,
					color: m.color || '',
				})
			}
		}
	}
	return methods
})

// Current plan details (matched from plans list)
const currentPlanDetail = computed(() => {
	if (!currentPlan.value || !plans.value.length) return null
	return plans.value.find((p: any) => p.id === currentPlan.value.plan_id) || null
})

// Balance warning
const isLowBalance = computed(() => {
	if (!wallet.value) return false
	return wallet.value.available_balance <= (wallet.value.warning_threshold || 0)
})

async function fetchWallet() {
	try {
		const res: any = await request.get('/tenant/wallet')
		wallet.value = res.data?.data
	} catch {
		wallet.value = null
	}
}

async function fetchPaymentInfo() {
	try {
		const res: any = await request.get('/tenant/payment-info')
		paymentInfo.value = res.data?.data
	} catch {
		paymentInfo.value = null
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

function selectPayMethod(method: { channel: string; type: string }) {
	selectedChannel.value = method.channel
	selectedPaymentMethod.value = method.type
}

async function handleRecharge() {
	if (!rechargeAmount.value || rechargeAmount.value <= 0) return
	if (!selectedChannel.value || !selectedPaymentMethod.value) return

	rechargeLoading.value = true
	try {
		const res: any = await request.post('/tenant/recharge/create', {
			amount: rechargeAmount.value,
			payment_channel: selectedChannel.value,
			payment_method: selectedPaymentMethod.value,
		})
		const data = res.data?.data
		if (data?.payment_url) {
			window.location.href = data.payment_url
			return
		}
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
	fetchPaymentInfo()

	// Handle pay result from return URL
	const pay = route.query.pay as string
	if (pay === 'success' || pay === 'fail') {
		payResult.value = pay
		setTimeout(() => { payResult.value = '' }, 5000)
		if (pay === 'success') {
			fetchWallet()
		}
	}
})

onBeforeUnmount(() => {
	if (frozenTimer) {
		clearInterval(frozenTimer)
	}
})
</script>

<template>
	<div class="wallet-page space-y-8">
		<!-- Page Header -->
		<div class="page-header">
			<h1 class="page-title">钱包</h1>
			<p class="page-description">管理余额、充值和套餐方案</p>
		</div>

		<!-- Pay Result Banner -->
		<transition name="fade">
			<div v-if="payResult === 'success'" class="pay-banner pay-banner-success">
				<div class="pay-banner-icon">
					<Icon name="checkCircle" size="sm" />
				</div>
				<div class="flex-1">
					<p class="text-sm font-semibold">充值成功</p>
					<p class="text-xs opacity-80 mt-0.5">余额已更新，请查看钱包余额</p>
				</div>
				<button class="opacity-60 hover:opacity-100 transition-opacity p-1" @click="payResult = ''">
					<Icon name="x" size="sm" />
				</button>
			</div>
			<div v-else-if="payResult === 'fail'" class="pay-banner pay-banner-fail">
				<div class="pay-banner-icon">
					<Icon name="xCircle" size="sm" />
				</div>
				<div class="flex-1">
					<p class="text-sm font-semibold">支付未完成</p>
					<p class="text-xs opacity-80 mt-0.5">如果已扣款请联系客服处理</p>
				</div>
				<button class="opacity-60 hover:opacity-100 transition-opacity p-1" @click="payResult = ''">
					<Icon name="x" size="sm" />
				</button>
			</div>
		</transition>

		<!-- ============================================ -->
		<!-- Hero Balance Card -->
		<!-- ============================================ -->
		<div class="balance-hero">
			<!-- Background decorations -->
			<div class="balance-hero-bg">
				<div class="balance-hero-orb balance-hero-orb-1"></div>
				<div class="balance-hero-orb balance-hero-orb-2"></div>
				<div class="balance-hero-orb balance-hero-orb-3"></div>
				<div class="balance-hero-grid"></div>
			</div>

			<div class="relative z-10 p-6 md:p-8 lg:p-10">
				<!-- Top row -->
				<div class="flex items-center justify-between mb-6">
					<div class="flex items-center gap-3">
						<div class="balance-hero-icon">
							<Icon name="wallet" size="md" />
						</div>
						<div>
							<p class="text-sm font-medium text-white/60">可用余额</p>
						</div>
					</div>
					<div class="balance-hero-currency">
						{{ wallet?.currency || 'USD' }}
					</div>
				</div>

				<!-- Balance number -->
				<div class="balance-hero-amount">
					<span class="balance-hero-dollar">$</span>
					<span class="balance-hero-value">{{ wallet?.available_balance?.toFixed(2) ?? '0.00' }}</span>
				</div>

				<!-- Low balance warning -->
				<div v-if="isLowBalance && wallet" class="balance-warning">
					<Icon name="exclamationTriangle" size="xs" />
					<span>余额低于预警线 ${{ wallet.warning_threshold?.toFixed(2) }}</span>
				</div>

				<!-- Secondary balances -->
				<div class="flex items-center gap-3 mt-5 flex-wrap">
					<button class="balance-chip group" @click="openFrozenModal">
						<span class="balance-chip-dot bg-amber-400"></span>
						<span class="text-white/50">冻结</span>
						<span class="text-white font-semibold">${{ wallet?.frozen_balance?.toFixed(2) ?? '0.00' }}</span>
						<Icon v-if="wallet?.frozen_balance > 0" name="chevronRight" size="xs"
							class="text-white/30 group-hover:text-white/50 transition-colors" />
					</button>

					<div class="w-px h-4 bg-white/10"></div>

					<div class="balance-chip-static">
						<span class="balance-chip-dot bg-white/30"></span>
						<span class="text-white/50">总余额</span>
						<span class="text-white font-semibold">${{ wallet?.balance?.toFixed(2) ?? '0.00' }}</span>
					</div>
				</div>
			</div>
		</div>

		<!-- ============================================ -->
		<!-- Two-column: Recharge + Current Plan -->
		<!-- ============================================ -->
		<div class="grid grid-cols-1 lg:grid-cols-5 gap-6">
			<!-- Recharge Card -->
			<div class="lg:col-span-3 card overflow-hidden">
				<div class="card-header">
					<div class="flex items-center gap-2.5">
						<div class="h-8 w-8 rounded-lg bg-gradient-to-br from-primary-100 to-primary-50 flex items-center justify-center">
							<Icon name="plus" size="sm" class="text-primary-600" />
						</div>
						<h2 class="font-semibold text-gray-900">充值</h2>
					</div>
				</div>

				<div class="card-body space-y-6">
					<!-- Amount Selection -->
					<div>
						<label class="input-label">充值金额（元）</label>
						<div class="grid grid-cols-2 sm:grid-cols-4 gap-2.5 mt-2">
							<button
								v-for="amt in presetAmounts"
								:key="amt"
								class="amount-pill"
								:class="rechargeAmount === amt ? 'amount-pill-active' : ''"
								@click="selectPresetAmount(amt)"
							>
								<span class="amount-pill-symbol">¥</span>{{ amt }}
							</button>
						</div>
						<div class="mt-3">
							<input
								v-model="customAmount"
								type="number"
								class="input"
								placeholder="自定义金额"
								min="1"
								step="0.01"
								@input="onCustomInput"
							/>
						</div>
					</div>

					<!-- Payment Method Selection -->
					<div v-if="payMethods.length > 0">
						<label class="input-label">支付方式</label>
						<div class="grid grid-cols-2 sm:grid-cols-3 gap-2.5 mt-2">
							<button
								v-for="method in payMethods"
								:key="method.channel + '-' + method.type"
								class="pay-method-card"
								:class="selectedChannel === method.channel && selectedPaymentMethod === method.type
									? 'pay-method-card-active'
									: ''"
								@click="selectPayMethod(method)"
							>
								<span
									class="pay-method-icon"
									:class="{
										'pay-method-icon-alipay': method.type === 'alipay',
										'pay-method-icon-wxpay': method.type === 'wxpay',
										'pay-method-icon-default': method.type !== 'alipay' && method.type !== 'wxpay',
									}"
								>
									<template v-if="method.type === 'alipay'">支</template>
									<template v-else-if="method.type === 'wxpay'">微</template>
									<template v-else>{{ method.name.substring(0, 1) }}</template>
								</span>
								<span class="text-sm font-medium text-gray-700">{{ method.name }}</span>
							</button>
						</div>
					</div>
					<div v-else class="py-6 text-center">
						<p class="text-sm text-gray-400">暂无可用的支付渠道，请联系管理员配置</p>
					</div>

					<!-- Submit -->
					<button
						class="btn btn-primary btn-lg w-full"
						:disabled="!rechargeAmount || !selectedChannel || !selectedPaymentMethod || rechargeLoading"
						@click="handleRecharge"
					>
						<template v-if="rechargeLoading">
							<span class="spinner"></span>
							处理中...
						</template>
						<template v-else>
							充值 ¥{{ rechargeAmount || '—' }}
						</template>
					</button>
				</div>
			</div>

			<!-- Current Plan Card -->
			<div class="lg:col-span-2">
				<div v-if="currentPlanDetail" class="card h-full flex flex-col">
					<div class="card-header">
						<div class="flex items-center gap-2.5">
							<div class="h-8 w-8 rounded-lg bg-gradient-to-br from-emerald-100 to-emerald-50 flex items-center justify-center">
								<Icon name="shield" size="sm" class="text-emerald-600" />
							</div>
							<h2 class="font-semibold text-gray-900">当前套餐</h2>
						</div>
						<span class="badge badge-success">使用中</span>
					</div>
					<div class="card-body flex-1 flex flex-col">
						<h3 class="text-lg font-semibold text-gray-900 mb-1">{{ currentPlanDetail.name }}</h3>
						<p class="text-xs text-gray-400 mb-5">{{ currentPlanDetail.description || '标准套餐方案' }}</p>

						<div class="space-y-3 flex-1">
							<div class="plan-detail-row">
								<span class="text-gray-500">月费</span>
								<span class="font-semibold text-gray-900">¥{{ Number(currentPlanDetail.monthly_price).toFixed(0) }}<span class="text-gray-400 font-normal text-xs">/月</span></span>
							</div>
							<div class="plan-detail-row">
								<span class="text-gray-500">Token 额度</span>
								<span v-if="currentPlanDetail.monthly_quota_tokens > 0" class="font-medium text-gray-700">
									{{ currentPlanDetail.monthly_quota_tokens.toLocaleString() }}/月
								</span>
								<span v-else class="font-medium text-primary-600">不限</span>
							</div>
							<div v-if="currentPlan.value?.expires_at" class="plan-detail-row">
								<span class="text-gray-500">到期时间</span>
								<span class="font-medium text-gray-700">
									{{ new Date(currentPlan.value.expires_at * 1000).toLocaleDateString('zh-CN') }}
								</span>
							</div>
						</div>

						<div class="mt-5 pt-4 border-t border-gray-100">
							<a href="#plans" class="text-sm text-primary-600 font-medium hover:text-primary-700 transition-colors inline-flex items-center gap-1">
								更换套餐
								<Icon name="chevronRight" size="xs" />
							</a>
						</div>
					</div>
				</div>

				<!-- No current plan -->
				<div v-else class="card h-full">
					<div class="card-header">
						<div class="flex items-center gap-2.5">
							<div class="h-8 w-8 rounded-lg bg-gray-50 flex items-center justify-center">
								<Icon name="exclamationCircle" size="sm" class="text-gray-400" />
							</div>
							<h2 class="font-semibold text-gray-900">当前套餐</h2>
						</div>
					</div>
					<div class="card-body flex flex-col items-center justify-center py-12">
						<div class="w-14 h-14 rounded-2xl bg-gray-50 flex items-center justify-center mb-4">
							<Icon name="creditCard" size="lg" class="text-gray-300" />
						</div>
						<p class="text-sm text-gray-500 mb-1">暂无订阅套餐</p>
						<a href="#plans" class="text-sm text-primary-600 font-medium hover:text-primary-700 transition-colors">
							查看可用套餐 &rarr;
						</a>
					</div>
				</div>
			</div>
		</div>

		<!-- ============================================ -->
		<!-- Subscription Plans -->
		<!-- ============================================ -->
		<div id="plans" class="card">
			<div class="card-header">
				<div class="flex items-center gap-2.5">
					<div class="h-8 w-8 rounded-lg bg-gradient-to-br from-violet-100 to-violet-50 flex items-center justify-center">
						<Icon name="chart" size="sm" class="text-violet-600" />
					</div>
					<div>
						<h2 class="font-semibold text-gray-900">套餐方案</h2>
						<p class="text-xs text-gray-400 mt-0.5">选择适合您团队的方案</p>
					</div>
				</div>
			</div>
			<div class="card-body">
				<!-- Loading -->
				<div v-if="loading" class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
					<div v-for="i in 3" :key="i" class="h-52 rounded-2xl border border-gray-100 p-5 animate-pulse">
						<div class="h-4 bg-gray-100 rounded w-1/3 mb-3"></div>
						<div class="h-8 bg-gray-100 rounded w-2/3 mb-4"></div>
						<div class="h-3 bg-gray-100 rounded w-full mb-2"></div>
						<div class="h-3 bg-gray-100 rounded w-3/4"></div>
					</div>
				</div>

				<!-- Empty -->
				<div v-else-if="plans.length === 0" class="py-12 text-center">
					<div class="inline-flex items-center justify-center w-14 h-14 rounded-2xl bg-gray-50 mb-4">
						<Icon name="exclamationCircle" size="lg" class="text-gray-300" />
					</div>
					<p class="text-gray-500 text-sm">暂无可用套餐，请联系管理员配置</p>
				</div>

				<!-- Plan Cards -->
				<div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
					<div
						v-for="plan in plans"
						:key="plan.id"
						class="plan-card group"
						:class="{
							'plan-card-recommended': plan.is_recommended,
							'plan-card-current': isCurrentPlan(plan),
						}"
					>
						<!-- Recommended accent bar -->
						<div v-if="plan.is_recommended" class="plan-card-accent"></div>

						<div class="p-5">
							<!-- Header row -->
							<div class="flex items-start justify-between mb-4">
								<h3 class="text-base font-semibold text-gray-900">{{ plan.name }}</h3>
								<div class="flex items-center gap-1.5">
									<span v-if="plan.is_recommended && !isCurrentPlan(plan)" class="badge badge-primary text-xs">推荐</span>
									<span v-if="isCurrentPlan(plan)" class="badge badge-success text-xs">当前</span>
								</div>
							</div>

							<!-- Price -->
							<div class="mb-5">
								<div class="flex items-baseline gap-1">
									<span class="text-xs text-gray-400">¥</span>
									<span class="text-3xl font-bold text-gray-900 tracking-tight">
										{{ Number(plan.monthly_price) === 0 ? '免费' : Number(plan.monthly_price).toFixed(0) }}
									</span>
									<span v-if="Number(plan.monthly_price) > 0" class="text-sm text-gray-400">/月</span>
								</div>
								<p v-if="Number(plan.yearly_price) > 0" class="text-xs text-primary-600 mt-1.5 font-medium">
									年付 ¥{{ Number(plan.yearly_price).toFixed(0) }}，省 ¥{{ (Number(plan.monthly_price) * 12 - Number(plan.yearly_price)).toFixed(0) }}
								</p>
							</div>

							<!-- Features -->
							<div class="space-y-2.5 mb-5">
								<div class="flex items-center gap-2.5">
									<div class="plan-card-check">
										<Icon name="check" size="xs" />
									</div>
									<span class="text-sm text-gray-600">
										{{ plan.monthly_quota_tokens > 0 ? plan.monthly_quota_tokens.toLocaleString() + ' Tokens/月' : '不限 Token 用量' }}
									</span>
								</div>
								<div v-if="plan.description" class="flex items-center gap-2.5">
									<div class="plan-card-check">
										<Icon name="check" size="xs" />
									</div>
									<span class="text-sm text-gray-600 line-clamp-1">{{ plan.description }}</span>
								</div>
							</div>

							<!-- Action -->
							<button
								v-if="isCurrentPlan(plan)"
								class="btn btn-secondary w-full btn-sm"
								disabled
							>
								当前方案
							</button>
							<button
								v-else
								class="btn w-full btn-sm"
								:class="plan.is_recommended ? 'btn-primary' : 'btn-secondary hover:!border-primary-300 hover:!text-primary-600'"
								@click="openConfirm(plan)"
							>
								{{ Number(plan.monthly_price) === 0 ? '免费开通' : '立即购买' }}
							</button>
						</div>
					</div>
				</div>
			</div>
		</div>

		<!-- ============================================ -->
		<!-- Confirm Subscribe Modal -->
		<!-- ============================================ -->
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
							<div v-if="selectedPlan" class="space-y-5">
								<!-- Selected plan summary -->
								<div class="flex items-center justify-between p-4 bg-gray-50 rounded-xl">
									<div>
										<p class="font-medium text-gray-900">{{ selectedPlan.name }}</p>
										<p class="text-xs text-gray-400 mt-0.5">{{ selectedPlan.identifier }}</p>
									</div>
									<span class="badge badge-primary">{{ selectedPlan.identifier }}</span>
								</div>

								<!-- Duration selection -->
								<div>
									<label class="input-label">订阅时长</label>
									<div class="grid grid-cols-2 gap-2 mt-2">
										<button
											v-for="opt in monthsOptions"
											:key="opt.value"
											class="rounded-xl px-3 py-2.5 text-sm font-medium border-2 transition-all"
											:class="selectedMonths === opt.value
												? 'border-primary-500 bg-primary-50 text-primary-700'
												: 'border-gray-200 text-gray-600 hover:border-gray-300'"
											@click="selectedMonths = opt.value"
										>
											{{ opt.label }}
										</button>
									</div>
								</div>

								<!-- Price summary -->
								<div class="flex items-center justify-between p-4 rounded-xl bg-gradient-to-r from-primary-50 to-primary-100/50 border border-primary-200/50">
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
								<template v-if="confirmLoading">
									<span class="spinner"></span>
									处理中...
								</template>
								<template v-else>确认订阅</template>
							</button>
						</div>
					</div>
				</div>
			</transition>
		</Teleport>

		<!-- ============================================ -->
		<!-- Frozen Items Modal -->
		<!-- ============================================ -->
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
								<div v-for="i in 3" :key="i" class="h-14 bg-gray-100 rounded-xl animate-pulse"></div>
							</div>
							<!-- Empty -->
							<div v-else-if="frozenItems.length === 0" class="py-10 text-center">
								<div class="inline-flex items-center justify-center w-12 h-12 rounded-full bg-gray-50 mb-3">
									<Icon name="checkCircle" size="lg" class="text-gray-300" />
								</div>
								<p class="text-gray-500 text-sm">当前没有冻结的资金</p>
							</div>
							<!-- Items list -->
							<div v-else class="space-y-2">
								<div v-for="item in frozenItems" :key="item.request_id"
									class="flex items-center justify-between p-3.5 bg-gray-50/80 rounded-xl hover:bg-gray-100/80 transition-colors">
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
							<p class="text-xs text-gray-400 flex-1 flex items-center gap-1.5">
								<span class="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse"></span>
								自动刷新中 · 每 10 秒更新
							</p>
							<button @click="closeFrozenModal" class="btn btn-secondary btn-sm">关闭</button>
						</div>
					</div>
				</div>
			</transition>
		</Teleport>
	</div>
</template>

<style scoped>
/* ==========================================
   Pay Result Banners
   ========================================== */
.pay-banner {
	border-radius: 1rem;
	padding: 0.875rem 1.25rem;
	display: flex;
	align-items: center;
	gap: 0.75rem;
}
.pay-banner-success {
	background-color: #ecfdf5;
	border: 1px solid rgba(167, 243, 208, 0.6);
	color: #065f46;
}
.pay-banner-success .pay-banner-icon {
	height: 2rem; width: 2rem; border-radius: 9999px;
	background-color: #d1fae5;
	display: flex; align-items: center; justify-content: center;
	flex-shrink: 0; color: #059669;
}
.pay-banner-fail {
	background-color: #fef2f2;
	border: 1px solid rgba(254, 202, 202, 0.6);
	color: #991b1b;
}
.pay-banner-fail .pay-banner-icon {
	height: 2rem; width: 2rem; border-radius: 9999px;
	background-color: #fee2e2;
	display: flex; align-items: center; justify-content: center;
	flex-shrink: 0; color: #dc2626;
}

/* ==========================================
   Hero Balance Card
   ========================================== */
.balance-hero {
	position: relative; overflow: hidden;
	border-radius: 1rem;
	background: linear-gradient(135deg, #0f766e 0%, #115e59 40%, #042f2e 100%);
}
.balance-hero-bg {
	position: absolute; inset: 0;
	pointer-events: none; overflow: hidden;
}
.balance-hero-orb { position: absolute; border-radius: 9999px; }
.balance-hero-orb-1 {
	top: -5rem; right: -4rem; height: 18rem; width: 18rem;
	background: radial-gradient(circle, rgba(45, 212, 191, 0.15) 0%, transparent 70%);
}
.balance-hero-orb-2 {
	bottom: -3rem; left: -3rem; height: 14rem; width: 14rem;
	background: radial-gradient(circle, rgba(94, 234, 212, 0.1) 0%, transparent 70%);
}
.balance-hero-orb-3 {
	top: 33%; right: 25%; height: 8rem; width: 8rem;
	background: radial-gradient(circle, rgba(20, 184, 166, 0.08) 0%, transparent 70%);
}
.balance-hero-grid {
	position: absolute; inset: 0;
	background-image: radial-gradient(circle at 1px 1px, rgba(255, 255, 255, 0.04) 1px, transparent 0);
	background-size: 24px 24px;
}
.balance-hero-icon {
	height: 2.5rem; width: 2.5rem; border-radius: 0.75rem;
	display: flex; align-items: center; justify-content: center;
	color: white;
	background: rgba(255, 255, 255, 0.1);
	border: 1px solid rgba(255, 255, 255, 0.08);
}
.balance-hero-currency {
	border-radius: 9999px; padding: 0.25rem 0.75rem;
	font-size: 0.75rem; font-weight: 600; letter-spacing: 0.025em;
	color: rgba(255, 255, 255, 0.5);
	background: rgba(255, 255, 255, 0.08);
	border: 1px solid rgba(255, 255, 255, 0.06);
}
.balance-hero-amount { display: flex; align-items: baseline; }
.balance-hero-dollar {
	font-size: 1.5rem; font-weight: 600; padding-right: 0.25rem;
	color: rgba(255, 255, 255, 0.4);
}
.balance-hero-value {
	font-size: 2.25rem; font-weight: 700; color: white;
	letter-spacing: -0.025em; font-variant-numeric: tabular-nums;
}
@media (min-width: 768px) {
	.balance-hero-value { font-size: 3rem; }
}
.balance-warning {
	margin-top: 0.75rem;
	display: inline-flex; align-items: center; gap: 0.375rem;
	padding: 0.375rem 0.75rem; border-radius: 0.5rem;
	font-size: 0.75rem; font-weight: 500; color: #fcd34d;
	background: rgba(251, 191, 36, 0.15);
	border: 1px solid rgba(251, 191, 36, 0.2);
}
.balance-chip {
	display: inline-flex; align-items: center; gap: 0.5rem;
	border-radius: 0.75rem; padding: 0.5rem 0.875rem;
	transition: all 0.2s;
	background: rgba(255, 255, 255, 0.06);
	border: 1px solid rgba(255, 255, 255, 0.04);
}
.balance-chip:hover { background: rgba(255, 255, 255, 0.1); }
.balance-chip-static {
	display: inline-flex; align-items: center; gap: 0.5rem;
	padding: 0 0.25rem;
}
.balance-chip-dot {
	height: 0.375rem; width: 0.375rem;
	border-radius: 9999px; flex-shrink: 0;
}

/* ==========================================
   Amount Pills
   ========================================== */
.amount-pill {
	position: relative; border-radius: 0.75rem;
	padding: 0.75rem 1rem; text-align: center;
	font-weight: 600; font-size: 0.875rem;
	transition: all 0.2s;
	border: 2px solid #e5e7eb; color: #374151;
}
.amount-pill:hover { border-color: #d1d5db; color: #111827; }
.amount-pill-active {
	border-color: #14b8a6; background-color: #f0fdfa; color: #0f766e;
	box-shadow: 0 0 0 1px rgba(20, 184, 166, 0.1), 0 0 16px rgba(20, 184, 166, 0.12);
}
.amount-pill-symbol {
	font-size: 0.75rem; font-weight: 500;
	margin-right: 0.125rem; opacity: 0.5;
}

/* ==========================================
   Payment Method Cards
   ========================================== */
.pay-method-card {
	display: flex; align-items: center; gap: 0.75rem;
	border-radius: 0.75rem; padding: 0.75rem 1rem;
	border: 2px solid #e5e7eb; transition: all 0.2s;
}
.pay-method-card:hover { border-color: #d1d5db; }
.pay-method-card-active {
	border-color: #14b8a6;
	background-color: rgba(240, 253, 250, 0.5);
	box-shadow: 0 0 0 1px rgba(20, 184, 166, 0.1), 0 0 16px rgba(20, 184, 166, 0.08);
}
.pay-method-icon {
	height: 2.25rem; width: 2.25rem; border-radius: 0.5rem;
	display: flex; align-items: center; justify-content: center;
	font-size: 0.75rem; font-weight: 700; color: white; flex-shrink: 0;
}
.pay-method-icon-alipay { background: linear-gradient(135deg, #1677ff, #0958d9); }
.pay-method-icon-wxpay { background: linear-gradient(135deg, #07c160, #06ad56); }
.pay-method-icon-default { background: linear-gradient(135deg, #6b7280, #4b5563); }

/* ==========================================
   Plan Detail Rows
   ========================================== */
.plan-detail-row {
	display: flex; align-items: center;
	justify-content: space-between;
	font-size: 0.875rem; padding: 0.25rem 0;
}

/* ==========================================
   Plan Cards
   ========================================== */
.plan-card {
	position: relative; border-radius: 1rem;
	border: 1px solid #e5e7eb; background: white;
	overflow: hidden; transition: all 0.3s;
}
.plan-card:hover {
	border-color: #d1d5db;
	box-shadow: 0 10px 40px rgba(0, 0, 0, 0.08);
}
.plan-card-recommended {
	border-color: #99f6e4;
	background: linear-gradient(180deg, rgba(240, 253, 250, 0.5) 0%, white 30%);
}
.plan-card-recommended:hover { border-color: #5eead4; }
.plan-card-current {
	border-color: #a7f3d0;
	background-color: rgba(236, 253, 245, 0.3);
}
.plan-card-current:hover { border-color: #6ee7b7; }
.plan-card-accent {
	height: 0.25rem; width: 100%;
	background: linear-gradient(90deg, #2dd4bf, #14b8a6, #0d9488);
}
.plan-card-check {
	height: 1.25rem; width: 1.25rem; border-radius: 0.375rem;
	background-color: #f0fdfa;
	display: flex; align-items: center; justify-content: center;
	flex-shrink: 0; color: #14b8a6;
}

/* ==========================================
   Transitions
   ========================================== */
.fade-enter-active { transition: all 0.3s ease-out; }
.fade-leave-active { transition: all 0.2s ease-in; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(-4px); }

/* ==========================================
   Page entrance animation
   ========================================== */
.wallet-page > * {
	animation: fade-in 0.4s ease-out both;
}
.wallet-page > *:nth-child(1) { animation-delay: 0ms; }
.wallet-page > *:nth-child(2) { animation-delay: 60ms; }
.wallet-page > *:nth-child(3) { animation-delay: 120ms; }
.wallet-page > *:nth-child(4) { animation-delay: 180ms; }
.wallet-page > *:nth-child(5) { animation-delay: 240ms; }
.wallet-page > *:nth-child(6) { animation-delay: 300ms; }

@keyframes fade-in {
	from { opacity: 0; transform: translateY(8px); }
	to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
	.wallet-page > * { animation: none !important; }
}
</style>
