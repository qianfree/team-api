<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput, NInputNumber, NModal } from 'naive-ui'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import request from '@/utils/request'
import { dispatchPayment } from '@/utils/payment'

const route = useRoute()
const wallet = ref<any>(null)
const walletLoading = ref(true)

// Recharge
const rechargeAmount = ref<number | null>(null)
const customAmount = ref<number | null>(null)
const rechargeLoading = ref(false)
const selectedChannel = ref('')
const selectedPaymentMethod = ref('')
const paymentInfo = ref<any>(null)
const paymentLoading = ref(true)

// Pay result notification
const payResult = ref<'success' | 'fail' | 'processing' | ''>('')

// Frozen items
const showFrozenModal = ref(false)
const frozenItems = ref<any[]>([])
const frozenLoading = ref(false)
let frozenTimer: ReturnType<typeof setInterval> | null = null

// Redeem
const showRedeemModal = ref(false)
const redeemCode = ref('')
const redeemLoading = ref(false)
const redeemResult = ref<any>(null)
const redeemHistory = ref<any[]>([])
const redeemHistoryLoading = ref(false)
const redeemTypeLabels: Record<string, string> = { quota: '额度', plan: '套餐', duration: '时长' }
const redeemTypeBadgeClasses: Record<string, string> = { quota: 'badge-success', plan: 'badge-primary', duration: 'badge-warning' }

// Warning threshold
const showThresholdModal = ref(false)
const thresholdInput = ref<number | null>(null)
const thresholdSaving = ref(false)

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

const minimumAmount = computed(() => Number(paymentInfo.value?.min_topup) || 1)

const thresholdActive = computed(() => Number(wallet.value?.warning_threshold) > 0)

const thresholdValidationError = computed(() => {
	const val = Number(thresholdInput.value)
	if (thresholdInput.value === null || isNaN(val)) return '请输入有效金额'
	if (val < 0) return '阈值不能为负数'
	return ''
})

const selectedDiscount = computed(() => {
	// 折扣档位仅整数金额精确命中（与后端 RechargeCreate 口径一致）：
	// 100.99 不享受 100 档折扣，避免前端展示与实际计价不符
	if (!rechargeAmount.value || !Number.isInteger(rechargeAmount.value)) return 1
	return Number(paymentInfo.value?.amount_discount?.[rechargeAmount.value]) || 1
})

const finalPayAmount = computed(() => (rechargeAmount.value || 0) * selectedDiscount.value)

const rechargeValidationMessage = computed(() => {
	if (!rechargeAmount.value) return ''
	if (rechargeAmount.value < minimumAmount.value) return `最低充值金额为 ¥${minimumAmount.value.toFixed(2)}`
	return ''
})

const rechargeReady = computed(() =>
	!!rechargeAmount.value
	&& rechargeAmount.value >= minimumAmount.value
	&& !!selectedChannel.value
	&& !!selectedPaymentMethod.value,
)

async function fetchWallet() {
	walletLoading.value = true
	try {
		const res: any = await request.get('/tenant/wallet')
		wallet.value = res.data?.data
	} catch {
		wallet.value = null
	} finally {
		walletLoading.value = false
	}
}

async function fetchPaymentInfo() {
	paymentLoading.value = true
	try {
		const res: any = await request.get('/tenant/payment-info')
		paymentInfo.value = res.data?.data
	} catch {
		paymentInfo.value = null
	} finally {
		paymentLoading.value = false
		const firstMethod = payMethods.value[0]
		if (firstMethod && !selectedPaymentMethod.value) {
			selectPayMethod(firstMethod)
		}
	}
}

function selectPresetAmount(amount: number) {
	rechargeAmount.value = amount
	customAmount.value = null
}

function onCustomInput() {
	const val = customAmount.value ?? 0
	rechargeAmount.value = val <= 0 ? null : val
}

function selectPayMethod(method: { channel: string; type: string }) {
	selectedChannel.value = method.channel
	selectedPaymentMethod.value = method.type
}

function discountText(amount: number): string {
	const ratio = Number(paymentInfo.value?.amount_discount?.[amount]) || 1
	return ratio < 1 ? `省 ${Math.round((1 - ratio) * 100)}%` : ''
}

async function handleRecharge() {
	if (!rechargeReady.value || !rechargeAmount.value) return

	rechargeLoading.value = true
	try {
		const res: any = await request.post('/tenant/recharge/create', {
			amount: rechargeAmount.value,
			payment_channel: selectedChannel.value,
			payment_method: selectedPaymentMethod.value,
		})
		const data = res.data?.data
		if (data?.payment_url) {
			dispatchPayment(data)
			return
		}
	} catch {
		// interceptor handles error toast
	} finally {
		rechargeLoading.value = false
	}
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

async function openRedeemModal() {
	showRedeemModal.value = true
	redeemCode.value = ''
	redeemResult.value = null
	await fetchRedeemHistory()
}

function closeRedeemModal() {
	showRedeemModal.value = false
}

async function handleRedeem() {
	if (!redeemCode.value.trim()) return
	redeemLoading.value = true
	redeemResult.value = null
	try {
		const res: any = await request.post('/tenant/redemptions/redeem', { code: redeemCode.value.trim() })
		redeemResult.value = res.data?.data
		redeemCode.value = ''
		await fetchRedeemHistory()
		fetchWallet()
	} catch {
		// interceptor handles error toast
	} finally {
		redeemLoading.value = false
	}
}

async function fetchRedeemHistory() {
	redeemHistoryLoading.value = true
	try {
		const res: any = await request.get('/tenant/redemptions/usages', {
			params: { page: 1, page_size: 10 },
		})
		redeemHistory.value = res.data?.data?.list || []
	} catch {
		redeemHistory.value = []
	} finally {
		redeemHistoryLoading.value = false
	}
}

// 兑换记录表格列
const redeemHistoryColumns = computed<DataTableColumns<any>>(() => [
	{
		title: '兑换码',
		key: 'code',
		width: 180,
		render: (row) => h('span', { class: 'font-mono text-xs' }, row.code || '-'),
	},
	{
		title: '兑换类型',
		key: 'type',
		width: 110,
		render: (row) => renderBadge(row.type, redeemTypeLabels, redeemTypeBadgeClasses),
	},
	{
		title: '面值',
		key: 'value',
		width: 130,
		render: (row) =>
			row.type === 'quota'
				? h('span', { class: 'font-mono' }, `+${Number(row.value).toFixed(6)}`)
				: h('span', { class: 'font-mono' }, '-'),
	},
	{
		title: '时间',
		key: 'created_at',
		width: 170,
		render: (row) => h('span', { class: 'text-gray-400 text-xs' }, row.created_at?.substring(0, 16)),
	},
])

// Warning threshold
function openThresholdModal() {
	thresholdInput.value = wallet.value?.warning_threshold ? Number(wallet.value.warning_threshold) : 0
	showThresholdModal.value = true
}

function closeThresholdModal() {
	showThresholdModal.value = false
}

async function saveThreshold() {
	if (thresholdValidationError.value) return
	const threshold = Number(thresholdInput.value)
	thresholdSaving.value = true
	try {
		await request.put('/tenant/wallet/warning-threshold', { threshold })
		// 拦截器在 code !== 0 时已 reject 并提示错误，走到这里即成功；更新本地展示
		if (wallet.value) wallet.value.warning_threshold = threshold
		showThresholdModal.value = false
	} catch {
		// 拦截器已提示错误
	} finally {
		thresholdSaving.value = false
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
	fetchPaymentInfo()

	// Handle pay result from return URL
	const pay = route.query.pay as string
	if (pay === 'success' || pay === 'fail' || pay === 'processing') {
		payResult.value = pay
		if (pay === 'success') {
			setTimeout(() => { payResult.value = '' }, 5000)
			fetchWallet()
		} else if (pay === 'processing') {
			// 异步回调可能仍在途：延迟刷新余额，到账后自动更新；超时后提示联系客服
			fetchWallet()
			setTimeout(fetchWallet, 3000)
			setTimeout(fetchWallet, 8000)
			setTimeout(() => { payResult.value = '' }, 12000)
		} else {
			setTimeout(() => { payResult.value = '' }, 5000)
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
	<div class="wallet-page space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<div>
				<h1 class="page-title">钱包</h1>
				<p class="page-description">查看余额并为账户充值</p>
			</div>
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
			<div v-else-if="payResult === 'processing'" class="pay-banner pay-banner-processing">
				<div class="pay-banner-icon">
					<Icon name="infoCircle" size="sm" />
				</div>
				<div class="flex-1">
					<p class="text-sm font-semibold">支付结果确认中</p>
					<p class="text-xs opacity-80 mt-0.5">正在等待到账，余额会在确认后自动更新</p>
				</div>
				<button class="opacity-60 hover:opacity-100 transition-opacity p-1" @click="payResult = ''">
					<Icon name="x" size="sm" />
				</button>
			</div>
		</transition>

		<section class="grid grid-cols-1 gap-5 lg:grid-cols-[minmax(280px,.72fr)_minmax(0,1.28fr)]">
			<div class="card wallet-card balance-card p-5 sm:p-6">
				<div v-if="walletLoading" class="space-y-5">
					<div class="flex items-center justify-between">
						<div class="skeleton h-10 w-36 rounded-xl"></div>
						<div class="skeleton h-6 w-14 rounded-full"></div>
					</div>
					<div class="skeleton h-12 w-52 rounded-xl"></div>
					<div class="skeleton h-16 rounded-xl"></div>
				</div>

				<template v-else>
					<div class="flex items-start justify-between gap-4">
						<div class="flex items-center gap-3">
							<div class="balance-icon"><Icon name="wallet" size="md" /></div>
							<div>
								<h2 class="text-base font-semibold text-slate-900">账户余额</h2>
								<p class="mt-0.5 text-xs text-slate-400">当前可用于模型调用</p>
							</div>
						</div>
						<span class="currency-badge">{{ wallet?.currency || 'USD' }}</span>
					</div>

					<div class="mt-8">
						<p class="text-xs font-medium text-slate-400">余额</p>
						<div class="mt-1 flex items-baseline gap-1.5">
							<span class="text-xl font-semibold text-slate-400">$</span>
							<strong class="balance-value">{{ wallet?.balance?.toFixed(2) ?? '0.00' }}</strong>
						</div>
					</div>

					<button class="frozen-balance-row mt-8" @click="openFrozenModal">
						<div class="flex items-center gap-2.5">
							<span class="h-2 w-2 rounded-full bg-amber-400"></span>
							<span class="text-sm text-slate-500">冻结金额</span>
						</div>
						<div class="flex items-center gap-2">
							<strong class="text-sm font-semibold tabular-nums text-slate-700">${{ wallet?.frozen_balance?.toFixed(2) ?? '0.00' }}</strong>
							<Icon name="chevronRight" size="xs" class="text-slate-300" />
						</div>
					</button>

					<button class="threshold-row mt-3" @click="openThresholdModal">
						<div class="flex items-center gap-2.5">
							<span class="h-2 w-2 rounded-full" :class="thresholdActive ? 'bg-primary-400' : 'bg-slate-300'"></span>
							<span class="text-sm text-slate-500">余额预警</span>
						</div>
						<div class="flex items-center gap-2">
							<strong v-if="thresholdActive" class="text-sm font-semibold tabular-nums text-slate-700">${{ Number(wallet?.warning_threshold).toFixed(2) }}</strong>
							<span v-else class="text-sm text-slate-400">已关闭</span>
							<Icon name="chevronRight" size="xs" class="text-slate-300" />
						</div>
					</button>

					<!-- 支付说明（管理员配置，支持纯文本或 HTML） -->
					<div v-if="paymentInfo?.payment_notice" class="payment-notice mt-3">
						<Icon name="infoCircle" size="xs" class="mt-0.5 flex-shrink-0" />
						<!-- 内容由平台管理员配置，信任来源，支持 HTML 渲染 -->
						<div v-html="paymentInfo.payment_notice" class="payment-notice-content"></div>
					</div>
				</template>
			</div>

			<div class="card wallet-card overflow-hidden">
				<div class="border-b border-slate-100/80 px-5 py-4 sm:px-6">
					<div class="flex items-center justify-between gap-3">
						<div class="flex min-w-0 items-center gap-3">
							<div class="recharge-icon"><Icon name="plus" size="md" /></div>
							<div class="min-w-0">
								<h2 class="text-base font-semibold text-slate-900">钱包充值</h2>
								<p class="mt-0.5 text-xs text-slate-400">选择金额和支付方式后前往收银台</p>
							</div>
						</div>
						<button class="btn btn-secondary btn-sm flex-shrink-0" @click="openRedeemModal">
							<Icon name="gift" size="sm" />
							<span class="hidden sm:inline">兑换码</span>
						</button>
					</div>
				</div>

				<div v-if="paymentLoading" class="space-y-5 p-5 sm:p-6">
					<div class="skeleton h-24 rounded-xl"></div>
					<div class="skeleton h-20 rounded-xl"></div>
					<div class="skeleton h-12 rounded-xl"></div>
				</div>

				<div v-else class="p-5 sm:p-6">
					<div>
						<div class="flex items-center justify-between">
							<label class="text-xs font-semibold text-slate-600">充值金额</label>
							<span class="text-[11px] text-slate-400">最低 ¥{{ minimumAmount.toFixed(2) }}</span>
						</div>
						<div class="mt-3 grid grid-cols-2 gap-2.5 sm:grid-cols-4">
							<button
								v-for="amount in presetAmounts"
								:key="amount"
								class="amount-pill"
								:class="{ 'amount-pill-active': rechargeAmount === amount && !customAmount }"
								@click="selectPresetAmount(amount)"
							>
								<span><small>¥</small>{{ amount }}</span>
								<em v-if="discountText(amount)">{{ discountText(amount) }}</em>
							</button>
						</div>
						<div class="relative mt-3">
							<span class="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm font-semibold text-slate-400">¥</span>
							<n-input-number
								v-model:value="customAmount"
								:min="minimumAmount"
								:step="0.01"
								:status="rechargeValidationMessage ? 'error' : undefined"
								placeholder="输入其他金额"
								style="width: 100%; padding-left: 1.75rem"
								@update:value="onCustomInput"
							/>
						</div>
						<p v-if="rechargeValidationMessage" class="mt-1.5 text-xs text-red-500">{{ rechargeValidationMessage }}</p>
					</div>

					<div class="my-5 border-t border-slate-100"></div>

					<div>
						<label class="text-xs font-semibold text-slate-600">支付方式</label>
						<div v-if="payMethods.length" class="mt-3 grid grid-cols-1 gap-2.5 sm:grid-cols-3">
							<button
								v-for="method in payMethods"
								:key="`${method.channel}-${method.type}`"
								class="pay-method-card"
								:class="{ 'pay-method-card-active': selectedChannel === method.channel && selectedPaymentMethod === method.type }"
								@click="selectPayMethod(method)"
							>
								<span class="pay-method-icon" :class="{
									'pay-method-icon-alipay': method.type === 'alipay',
									'pay-method-icon-wxpay': method.type === 'wxpay',
									'pay-method-icon-default': method.type !== 'alipay' && method.type !== 'wxpay',
								}">
									{{ method.type === 'alipay' ? '支' : method.type === 'wxpay' ? '微' : method.name.substring(0, 1) }}
								</span>
								<span class="min-w-0 flex-1 truncate text-left text-sm font-medium text-slate-700">{{ method.name }}</span>
								<Icon v-if="selectedChannel === method.channel && selectedPaymentMethod === method.type" name="checkCircle" size="sm" class="text-primary-500" />
							</button>
						</div>
						<div v-else class="mt-3 rounded-xl border border-dashed border-slate-200 px-4 py-8 text-center text-sm text-slate-400">
							暂无可用支付渠道
						</div>
					</div>

					<div class="recharge-summary mt-5">
						<div class="min-w-0">
							<p class="text-[11px] text-slate-400">本次实付</p>
							<div class="mt-0.5 flex items-baseline gap-2">
								<strong class="text-xl font-bold tabular-nums text-slate-800">¥{{ finalPayAmount.toFixed(2) }}</strong>
								<span v-if="selectedDiscount < 1" class="text-xs text-slate-400 line-through">¥{{ rechargeAmount?.toFixed(2) }}</span>
							</div>
							<p v-if="selectedDiscount < 1" class="mt-0.5 text-[11px] text-emerald-600">到账仍按 ¥{{ rechargeAmount?.toFixed(2) }} 全额折算</p>
						</div>
						<button class="btn btn-primary min-w-32" :disabled="!rechargeReady || rechargeLoading" @click="handleRecharge">
							<Icon v-if="rechargeLoading" name="refresh" size="sm" class="animate-spin" />
							<Icon v-else name="creditCard" size="sm" />
							{{ rechargeLoading ? '处理中' : '确认充值' }}
						</button>
					</div>

					<div class="recharge-note mt-3">
						<Icon name="infoCircle" size="xs" class="mt-0.5 flex-shrink-0" />
						<p>支付金额为人民币，到账后按平台汇率换算为美元钱包余额。</p>
					</div>
				</div>
			</div>
		</section>

		<!-- ============================================ -->
		<!-- Frozen Items Modal -->
		<!-- ============================================ -->
		<n-modal
			v-model:show="showFrozenModal"
			preset="card"
			title="冻结明细"
			:style="{ width: '640px' }"
			@update:show="(v: boolean) => { if (!v) closeFrozenModal() }"
		>
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
			<template #footer>
				<div class="flex items-center justify-between gap-3">
					<p class="text-xs text-gray-400 flex-1 flex items-center gap-1.5">
						<span class="w-1.5 h-1.5 rounded-full bg-primary-400 animate-pulse"></span>
						自动刷新中 · 每 10 秒更新
					</p>
					<button @click="closeFrozenModal" class="btn btn-secondary btn-sm">关闭</button>
				</div>
			</template>
		</n-modal>

		<!-- ============================================ -->
		<!-- Redeem Code Modal -->
		<!-- ============================================ -->
		<n-modal
			v-model:show="showRedeemModal"
			preset="card"
			title="兑换码"
			:style="{ width: '640px' }"
			@update:show="(v: boolean) => { if (!v) closeRedeemModal() }"
		>
			<!-- Redeem input -->
			<div>
				<p class="text-sm text-gray-500 mb-3">输入兑换码领取额度、套餐时长等福利</p>
				<div class="flex gap-3">
					<div class="flex-1">
						<n-input
							v-model:value="redeemCode"
							type="text"
							class="font-mono"
							placeholder="请输入兑换码"
							:maxlength="32"
							@keyup.enter="handleRedeem"
						/>
					</div>
					<button
						class="btn btn-primary"
						:disabled="redeemLoading || !redeemCode.trim()"
						@click="handleRedeem"
					>
						<Icon v-if="redeemLoading" name="refresh" size="sm" class="animate-spin" />
						<Icon v-else name="check" size="sm" />
						{{ redeemLoading ? '兑换中...' : '兑换' }}
					</button>
				</div>

				<!-- Success -->
				<div v-if="redeemResult" class="mt-3 flex items-center gap-2 text-sm text-emerald-700 bg-emerald-50 rounded-lg px-3 py-2">
					<Icon name="checkCircle" size="sm" />
					兑换成功！
					<span v-if="redeemResult.type === 'quota'" class="font-medium">
						获得 {{ redeemResult.credited?.toLocaleString() }} 额度
					</span>
					<span v-else-if="redeemResult.type === 'plan'" class="font-medium">
						获得 {{ redeemResult.months }} 个月套餐
					</span>
					<span v-else-if="redeemResult.type === 'duration'" class="font-medium">
						账户有效期延长 {{ redeemResult.extended_days }} 天
					</span>
				</div>
			</div>

			<!-- Divider -->
			<div class="border-t border-gray-100"></div>

			<!-- Redeem history -->
			<div>
				<h4 class="text-sm font-semibold text-gray-900 mb-3">兑换记录</h4>

				<!-- History table -->
				<n-data-table
					:loading="redeemHistoryLoading"
					:columns="redeemHistoryColumns"
					:scroll-x="tableScrollX(redeemHistoryColumns)"
					:data="redeemHistory"
					:row-key="(row: any) => row.id"
				>
					<template #empty>
						<div class="flex flex-col items-center justify-center py-8 text-center">
							<div class="mb-3 text-gray-300">
								<Icon name="document" size="lg" />
							</div>
							<p class="text-sm text-gray-500">暂无兑换记录</p>
						</div>
					</template>
				</n-data-table>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button @click="closeRedeemModal" class="btn btn-secondary btn-sm">关闭</button>
				</div>
			</template>
		</n-modal>

		<!-- ============================================ -->
		<!-- Warning Threshold Modal -->
		<!-- ============================================ -->
		<n-modal
			v-model:show="showThresholdModal"
			preset="card"
			title="余额预警"
			:style="{ width: '480px' }"
			@update:show="(v: boolean) => { if (!v) closeThresholdModal() }"
		>
			<div class="space-y-4">
				<div class="threshold-note">
					<Icon name="infoCircle" size="sm" class="mt-0.5 flex-shrink-0 text-primary-500" />
					<p>当可用余额低于预警线时，向组织 owner / admin 发送通知。设为 0 可关闭预警。</p>
				</div>
				<div>
					<label class="text-xs font-semibold text-slate-600">预警阈值（USD）</label>
					<div class="relative mt-3">
						<span class="absolute left-3.5 top-1/2 -translate-y-1/2 text-sm font-semibold text-slate-400">$</span>
						<n-input-number
							v-model:value="thresholdInput"
							:min="0"
							:step="0.01"
							:status="thresholdValidationError ? 'error' : undefined"
							placeholder="例如 1.00"
							style="width: 100%; padding-left: 1.75rem"
							@keyup.enter="saveThreshold"
						/>
					</div>
					<p v-if="thresholdValidationError" class="mt-1.5 text-xs text-red-500">{{ thresholdValidationError }}</p>
					<p v-else class="mt-1.5 text-xs text-slate-400">设为 0 表示关闭预警</p>
				</div>
				<div class="flex flex-wrap gap-2">
					<button type="button" class="threshold-quick" :class="{ 'threshold-quick-active': Number(thresholdInput) === 0 }" @click="thresholdInput = 0">关闭</button>
					<button type="button" class="threshold-quick" :class="{ 'threshold-quick-active': Number(thresholdInput) === 1 }" @click="thresholdInput = 1">$1</button>
					<button type="button" class="threshold-quick" :class="{ 'threshold-quick-active': Number(thresholdInput) === 5 }" @click="thresholdInput = 5">$5</button>
					<button type="button" class="threshold-quick" :class="{ 'threshold-quick-active': Number(thresholdInput) === 10 }" @click="thresholdInput = 10">$10</button>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button @click="closeThresholdModal" class="btn btn-secondary btn-sm">取消</button>
					<button class="btn btn-primary btn-sm" :disabled="thresholdSaving || !!thresholdValidationError" @click="saveThreshold">
						<Icon v-if="thresholdSaving" name="refresh" size="sm" class="animate-spin" />
						<Icon v-else name="check" size="sm" />
						{{ thresholdSaving ? '保存中...' : '保存' }}
					</button>
				</div>
			</template>
		</n-modal>
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
.pay-banner-processing {
	background-color: #fffbeb;
	border: 1px solid rgba(252, 211, 77, 0.6);
	color: #92400e;
}
.pay-banner-processing .pay-banner-icon {
	height: 2rem; width: 2rem; border-radius: 9999px;
	background-color: #fef3c7;
	display: flex; align-items: center; justify-content: center;
	flex-shrink: 0; color: #d97706;
}

/* 支付说明（管理员配置，支持 HTML） */
.payment-notice {
	display: flex;
	align-items: flex-start;
	gap: 0.5rem;
	border-radius: 0.75rem;
	background: #fffbeb;
	border: 1px solid rgba(252, 211, 77, 0.6);
	padding: 0.75rem 1rem;
	color: #92400e;
}
.payment-notice-content {
	flex: 1;
	min-width: 0;
	font-size: 0.8125rem;
	line-height: 1.6;
}
.payment-notice-content :deep(p) { margin: 0 0 0.5rem; }
.payment-notice-content :deep(p:last-child) { margin-bottom: 0; }
.payment-notice-content :deep(a) { color: inherit; text-decoration: underline; }
.payment-notice-content :deep(strong) { font-weight: 600; }

.wallet-card {
	background: rgba(255, 255, 255, 0.94);
}

.balance-icon,
.recharge-icon {
	display: flex;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	height: 2.5rem;
	width: 2.5rem;
	border-radius: 0.75rem;
}

.balance-icon {
	background: #eef2ff;
	color: #6558d9;
}

.recharge-icon {
	background: #ecfeff;
	color: #0891b2;
}

.currency-badge {
	flex-shrink: 0;
	border: 1px solid #e2e8f0;
	border-radius: 9999px;
	background: #f8fafc;
	padding: 0.25rem 0.55rem;
	color: #64748b;
	font-size: 0.6875rem;
	font-weight: 700;
}

.balance-value {
	max-width: 100%;
	overflow: hidden;
	color: #172033;
	font-size: 2.25rem;
	font-weight: 750;
	font-variant-numeric: tabular-nums;
	line-height: 1.1;
	text-overflow: ellipsis;
	white-space: nowrap;
}

.frozen-balance-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	width: 100%;
	border: 1px solid #e2e8f0;
	border-radius: 0.75rem;
	background: #f8fafc;
	padding: 0.875rem 1rem;
	transition: border-color 180ms ease, background-color 180ms ease;
}

.frozen-balance-row:hover {
	border-color: #fde68a;
	background: #fffbeb;
}

.threshold-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	width: 100%;
	border: 1px solid #e2e8f0;
	border-radius: 0.75rem;
	background: #f8fafc;
	padding: 0.875rem 1rem;
	transition: border-color 180ms ease, background-color 180ms ease;
}
.threshold-row:hover {
	border-color: #99f6e4;
	background: #f0fdfa;
}

.threshold-note {
	display: flex;
	align-items: flex-start;
	gap: 0.5rem;
	border-radius: 0.75rem;
	background: #f0fdfa;
	padding: 0.65rem 0.75rem;
	color: #0f766e;
}
.threshold-note p {
	font-size: 0.75rem;
	line-height: 1.5;
}

.threshold-quick {
	border: 1px solid #e2e8f0;
	border-radius: 0.5rem;
	background: rgba(255, 255, 255, 0.72);
	padding: 0.375rem 0.875rem;
	font-size: 0.8125rem;
	font-weight: 600;
	color: #475569;
	transition: border-color 180ms ease, background-color 180ms ease, color 180ms ease;
}
.threshold-quick:hover {
	border-color: #99f6e4;
	color: #0d9488;
}
.threshold-quick-active {
	border-color: #5eead4;
	background: #f0fdfa;
	color: #0d9488;
}

.amount-pill {
	display: flex;
	min-height: 3.75rem;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	border: 1px solid #e2e8f0;
	border-radius: 0.75rem;
	background: rgba(255, 255, 255, 0.72);
	color: #475569;
	font-size: 0.9375rem;
	font-weight: 700;
	transition: border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease, color 180ms ease;
}

.amount-pill small { margin-right: 0.125rem; font-size: 0.6875rem; font-weight: 500; }
.amount-pill em { margin-top: 0.15rem; color: #059669; font-size: 0.5625rem; font-style: normal; font-weight: 700; }
.amount-pill:hover { border-color: #99f6e4; color: #0d9488; }
.amount-pill-active {
	border-color: #5eead4;
	background: #f0fdfa;
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.08);
	color: #0d9488;
}

.pay-method-card {
	display: flex;
	min-width: 0;
	min-height: 3.5rem;
	align-items: center;
	gap: 0.625rem;
	border: 1px solid #e2e8f0;
	border-radius: 0.75rem;
	background: rgba(255, 255, 255, 0.72);
	padding: 0.625rem 0.75rem;
	transition: border-color 180ms ease, background-color 180ms ease, box-shadow 180ms ease;
}

.pay-method-card:hover { border-color: #99f6e4; }
.pay-method-card-active {
	border-color: #5eead4;
	background: #f0fdfa;
	box-shadow: 0 0 0 3px rgba(20, 184, 166, 0.08);
}

.pay-method-icon {
	display: flex;
	height: 2rem;
	width: 2rem;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	border-radius: 0.625rem;
	color: white;
	font-size: 0.6875rem;
	font-weight: 700;
}

.pay-method-icon-alipay { background: #1677ff; }
.pay-method-icon-wxpay { background: #07c160; }
.pay-method-icon-default { background: #64748b; }

.recharge-summary {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 1rem;
	border-top: 1px solid #f1f5f9;
	padding-top: 1.25rem;
}

.recharge-note {
	display: flex;
	align-items: flex-start;
	gap: 0.5rem;
	border-radius: 0.75rem;
	background: #f8fafc;
	padding: 0.65rem 0.75rem;
	color: #64748b;
}

.recharge-note p { font-size: 0.6875rem; line-height: 1.5; }

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

@keyframes fade-in {
	from { opacity: 0; transform: translateY(8px); }
	to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
	.wallet-page > * { animation: none !important; }
}
</style>
