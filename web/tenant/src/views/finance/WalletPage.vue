<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { dispatchPayment } from '@/utils/payment'

const route = useRoute()
const wallet = ref<any>(null)

// Recharge
const rechargeAmount = ref<number | null>(null)
const customAmount = ref('')
const rechargeLoading = ref(false)
const selectedChannel = ref('')
const selectedPaymentMethod = ref('')
const paymentInfo = ref<any>(null)

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

// Balance warning
const isLowBalance = computed(() => {
	if (!wallet.value) return false
	return wallet.value.available_balance <= (wallet.value.warning_threshold || 0)
})

// Warning threshold setting
const showThresholdModal = ref(false)
const thresholdInput = ref('')
const thresholdSaving = ref(false)

function openThresholdModal() {
	thresholdInput.value = wallet.value?.warning_threshold?.toFixed(2) ?? '1.00'
	showThresholdModal.value = true
}

async function saveThreshold() {
	const val = parseFloat(thresholdInput.value)
	if (isNaN(val) || val < 0) return
	thresholdSaving.value = true
	try {
		const res: any = await request.put('/tenant/wallet/warning-threshold', { threshold: val })
		if (res.data?.code === 0) {
			showThresholdModal.value = false
			await fetchWallet()
		}
	} catch (e) {
		console.error(e)
	} finally {
		thresholdSaving.value = false
	}
}

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

// Redeem
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
			params: { page: 1, page_size: 10 }
		})
		redeemHistory.value = res.data?.data?.list || []
	} catch {
		redeemHistory.value = []
	} finally {
		redeemHistoryLoading.value = false
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
		<div class="page-header flex items-start justify-between">
			<div>
				<h1 class="page-title">钱包</h1>
				<p class="page-description">管理余额与充值</p>
			</div>
			<button class="btn btn-secondary btn-sm" @click="openRedeemModal">
				<Icon name="gift" size="sm" />
				兑换码
			</button>
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

		<!-- ============================================ -->
		<!-- Two-column: Balance + Recharge -->
		<!-- ============================================ -->
		<div class="grid grid-cols-1 lg:grid-cols-12 gap-6">
			<!-- Balance Hero Card -->
			<div class="lg:col-span-5">
				<div class="balance-hero h-full">
					<!-- Background decorations -->
					<div class="balance-hero-bg">
						<div class="balance-hero-orb balance-hero-orb-1"></div>
						<div class="balance-hero-orb balance-hero-orb-2"></div>
						<div class="balance-hero-grid"></div>
					</div>

					<div class="relative z-10 p-6 md:p-8">
						<!-- Top row -->
						<div class="flex items-center justify-between mb-5">
							<div class="flex items-center gap-3">
								<div class="balance-hero-icon">
									<Icon name="wallet" size="md" />
								</div>
								<p class="text-sm font-medium text-white/60">可用余额</p>
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
							<button class="ml-2 underline underline-offset-2 opacity-70 hover:opacity-100" @click="openThresholdModal">修改</button>
						</div>

						<!-- Threshold setting (when not low balance) -->
						<div v-else-if="wallet" class="mt-3">
							<button class="balance-threshold-btn" @click="openThresholdModal">
								<Icon name="cog" size="xs" />
								<span>预警线: ${{ wallet.warning_threshold?.toFixed(2) || '0.00' }}</span>
							</button>
						</div>

						<!-- Secondary balances -->
						<div class="flex flex-col gap-3 mt-6">
							<button class="balance-chip group" @click="openFrozenModal">
								<div class="flex items-center gap-2.5">
									<span class="balance-chip-dot bg-amber-400"></span>
									<span class="text-white/50 text-sm">冻结金额</span>
								</div>
								<div class="flex items-center gap-1.5">
									<span class="text-white font-semibold text-sm">${{ wallet?.frozen_balance?.toFixed(2) ?? '0.00' }}</span>
									<Icon v-if="wallet?.frozen_balance > 0" name="chevronRight" size="xs"
										class="text-white/30 group-hover:text-white/50 transition-colors" />
								</div>
							</button>

							<div class="balance-chip-static">
								<div class="flex items-center gap-2.5">
									<span class="balance-chip-dot bg-white/20"></span>
									<span class="text-white/50 text-sm">总余额</span>
								</div>
								<span class="text-white font-semibold text-sm">${{ wallet?.balance?.toFixed(2) ?? '0.00' }}</span>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Recharge Card -->
			<div class="lg:col-span-7">
				<div class="card h-full">
					<div class="card-header">
						<div class="flex items-center gap-2.5">
							<div class="h-8 w-8 rounded-lg bg-gradient-to-br from-primary-100 to-primary-50 flex items-center justify-center">
								<Icon name="plus" size="sm" class="text-primary-600" />
							</div>
							<h2 class="font-semibold text-gray-900">充值</h2>
						</div>
					</div>

					<div class="card-body space-y-5">
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
			</div>
		</div>

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

		<!-- ============================================ -->
		<!-- Redeem Code Modal -->
		<!-- ============================================ -->
		<Teleport to="body">
			<transition name="modal">
				<div v-if="showRedeemModal" class="modal-overlay" @click.self="closeRedeemModal">
					<div class="modal-content w-full max-w-lg">
						<div class="modal-header">
							<h3 class="modal-title">兑换码</h3>
							<button @click="closeRedeemModal" class="btn-ghost btn-icon">
								<Icon name="x" size="md" />
							</button>
						</div>
						<div class="modal-body space-y-5">
							<!-- Redeem input -->
							<div>
								<p class="text-sm text-gray-500 mb-3">输入兑换码领取额度、套餐时长等福利</p>
								<div class="flex gap-3">
									<div class="flex-1">
										<input
											v-model="redeemCode"
											type="text"
											class="input font-mono"
											placeholder="请输入兑换码"
											maxlength="32"
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

								<!-- Loading -->
								<div v-if="redeemHistoryLoading" class="flex items-center justify-center py-8">
									<div class="spinner"></div>
									<span class="ml-2 text-sm text-gray-400">加载中...</span>
								</div>

								<!-- Empty -->
								<div v-else-if="redeemHistory.length === 0" class="flex flex-col items-center justify-center py-8 text-center">
									<div class="mb-3 text-gray-300">
										<Icon name="document" size="lg" />
									</div>
									<p class="text-sm text-gray-500">暂无兑换记录</p>
								</div>

								<!-- History table -->
								<div v-else class="table-container">
									<table class="table">
										<thead>
											<tr>
												<th>兑换码</th>
												<th>兑换类型</th>
												<th>面值</th>
												<th>时间</th>
											</tr>
										</thead>
										<tbody>
											<tr v-for="item in redeemHistory" :key="item.id">
												<td class="font-mono text-xs">{{ item.code || '-' }}</td>
												<td>
													<span class="badge" :class="redeemTypeBadgeClasses[item.type] || 'badge-gray'">
														{{ redeemTypeLabels[item.type] || item.type }}
													</span>
												</td>
												<td class="font-mono">
													<template v-if="item.type === 'quota'">
														+{{ Number(item.value).toFixed(6) }}
													</template>
													<template v-else>
														-
													</template>
												</td>
												<td class="text-gray-400 text-xs">{{ item.created_at?.substring(0, 16) }}</td>
											</tr>
										</tbody>
									</table>
								</div>
							</div>
						</div>
						<div class="modal-footer">
							<button @click="closeRedeemModal" class="btn btn-secondary btn-sm">关闭</button>
						</div>
					</div>
				</div>
			</transition>
		</Teleport>

		<!-- Warning Threshold Modal -->
		<Teleport to="body">
			<div v-if="showThresholdModal" class="modal-overlay" @click.self="showThresholdModal = false">
				<div class="modal-content bg-white w-full max-w-md">
					<div class="modal-header">
						<h3 class="modal-title">余额预警设置</h3>
						<button class="btn btn-ghost btn-icon" @click="showThresholdModal = false">
							<Icon name="x" size="sm" />
						</button>
					</div>
					<div class="modal-body space-y-4">
						<div class="rounded-xl bg-gray-50 border border-gray-200 p-4">
							<p class="text-sm text-gray-600">当可用余额低于设定的阈值时，系统将通过站内通知和 Webhook 推送预警消息。</p>
						</div>
						<div>
							<label class="input-label">预警阈值（USD）</label>
							<div class="relative mt-1">
								<span class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 text-sm">$</span>
								<input
									v-model="thresholdInput"
									type="number"
									step="0.01"
									min="0"
									class="input pl-7"
									placeholder="0.00"
								/>
							</div>
							<p class="input-hint">设为 0 表示关闭余额预警</p>
						</div>
					</div>
					<div class="modal-footer">
						<button class="btn btn-secondary" @click="showThresholdModal = false">取消</button>
						<button class="btn btn-primary" :disabled="thresholdSaving" @click="saveThreshold">
							<div v-if="thresholdSaving" class="spinner h-4 w-4 border-white"></div>
							{{ thresholdSaving ? '保存中...' : '保存' }}
						</button>
					</div>
				</div>
			</div>
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

/* ==========================================
   Balance Hero Card
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
@media (min-width: 1024px) {
	.balance-hero-value { font-size: 2.5rem; }
}
.balance-warning {
	margin-top: 0.75rem;
	display: inline-flex; align-items: center; gap: 0.375rem;
	padding: 0.375rem 0.75rem; border-radius: 0.5rem;
	font-size: 0.75rem; font-weight: 500; color: #fcd34d;
	background: rgba(251, 191, 36, 0.15);
	border: 1px solid rgba(251, 191, 36, 0.2);
}
.balance-threshold-btn {
	display: inline-flex; align-items: center; gap: 0.375rem;
	font-size: 0.75rem; font-weight: 500; color: rgba(255,255,255,0.5);
	transition: color 0.15s;
}
.balance-threshold-btn:hover { color: rgba(255,255,255,0.75); }
.balance-chip {
	display: flex; align-items: center; justify-content: space-between;
	border-radius: 0.75rem; padding: 0.625rem 0.875rem;
	transition: all 0.2s;
	background: rgba(255, 255, 255, 0.06);
	border: 1px solid rgba(255, 255, 255, 0.04);
	width: 100%;
}
.balance-chip:hover { background: rgba(255, 255, 255, 0.1); }
.balance-chip-static {
	display: flex; align-items: center; justify-content: space-between;
	padding: 0.625rem 0.875rem;
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
