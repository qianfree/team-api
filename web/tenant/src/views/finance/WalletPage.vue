<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const wallet = ref<any>(null)
const transactions = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/wallet/transactions/export',
})

// Recharge
const rechargeAmount = ref<number | null>(null)
const customAmount = ref('')
const rechargeLoading = ref(false)
const presetAmounts = [10, 50, 100, 500]

const txTypeLabel: Record<string, string> = {
	recharge: '充值',
	consume: '消费',
	pre_deduct: '预扣',
	settle: '结算',
	refund: '退款',
	adjust: '调整',
	freeze: '冻结',
	unfreeze: '解冻',
}

const txTypeBadgeClass: Record<string, string> = {
	recharge: 'badge-success',
	refund: 'badge-success',
	consume: 'badge-danger',
	pre_deduct: 'badge-danger',
	settle: 'badge-danger',
	adjust: 'badge-warning',
	freeze: 'badge-gray',
	unfreeze: 'badge-gray',
}

function formatAmount(amount: number): string {
	if (amount >= 0) return '+$' + amount.toFixed(2)
	return '-$' + Math.abs(amount).toFixed(2)
}

async function fetchWallet() {
	try {
		const res: any = await request.get('/tenant/wallet')
		wallet.value = res.data?.data
	} catch {
		wallet.value = null
	}
}

async function fetchTransactions() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/wallet/transactions', {
			params: { page: page.value, page_size: pageSize },
		})
		const raw = res.data?.data; transactions.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		transactions.value = []
	} finally {
		loading.value = false
	}
}

function prevPage() {
	if (page.value > 1) {
		page.value--
		fetchTransactions()
	}
}

function nextPage() {
	if (page.value * pageSize < total.value) {
		page.value++
		fetchTransactions()
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
		fetchTransactions()
	} catch (e) {
		// interceptor handles error toast
	} finally {
		rechargeLoading.value = false
	}
}

onMounted(() => {
	fetchWallet()
	fetchTransactions()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<h1 class="page-title">钱包</h1>
			<p class="page-description">查看余额和交易记录</p>
		</div>

		<!-- Wallet Summary -->
		<div class="grid grid-cols-1 sm:grid-cols-3 gap-5">
			<div class="card p-5">
				<p class="stat-label">可用余额</p>
				<p class="text-2xl font-bold text-gray-900">${{ wallet?.available_balance?.toFixed(2) ?? '0.00' }}</p>
				<p class="text-xs text-gray-400 mt-1">{{ wallet?.currency || 'USD' }}</p>
			</div>
			<div class="card p-5">
				<p class="stat-label">冻结余额</p>
				<p class="text-2xl font-bold text-amber-600">${{ wallet?.frozen_balance?.toFixed(2) ?? '0.00' }}</p>
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

		<!-- Transactions -->
		<div class="card">
			<div class="card-header">
				<div class="flex items-center justify-between w-full">
					<div>
						<h2 class="font-semibold text-gray-900">交易记录</h2>
						<p class="text-sm text-gray-500 mt-0.5">共 {{ total }} 条记录</p>
					</div>
					<!-- Export dropdown -->
					<div class="relative inline-block">
						<button class="btn btn-secondary btn-sm" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
							<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
							<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
							导出
						</button>
						<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
							<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
							<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
						</div>
					</div>
					<button class="btn btn-ghost btn-sm" @click="fetchTransactions">
						<Icon name="refresh" size="sm" />
						刷新
					</button>
				</div>
			</div>

			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="transactions.length > 0" class="table-container border-0 rounded-none">
				<table class="table">
					<thead>
						<tr>
							<th>类型</th>
							<th>金额</th>
							<th>余额</th>
							<th>描述</th>
							<th>时间</th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="tx in transactions" :key="tx.id">
							<td>
								<span class="badge" :class="txTypeBadgeClass[tx.type] || 'badge-gray'">
									{{ txTypeLabel[tx.type] || tx.type }}
								</span>
							</td>
							<td :class="tx.amount >= 0 ? 'text-emerald-600 font-semibold' : 'text-red-600 font-semibold'">
								{{ formatAmount(tx.amount) }}
							</td>
							<td class="text-gray-700">${{ tx.balance_after?.toFixed(2) ?? '--' }}</td>
							<td class="text-gray-500 text-sm">{{ tx.description || '--' }}</td>
							<td class="text-gray-500 text-xs">{{ (tx.created_at || '').replace('T', ' ').substring(0, 16) }}</td>
						</tr>
					</tbody>
				</table>
			</div>

			<div v-else class="empty-state">
				<Icon name="creditCard" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无交易记录</p>
				<p class="empty-state-description">交易记录将在 API 调用和充值后展示</p>
			</div>

			<!-- Pagination -->
			<div v-if="total > pageSize" class="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
				<p class="text-sm text-gray-500">
					第 {{ page }} / {{ Math.ceil(total / pageSize) }} 页
				</p>
				<div class="flex items-center gap-2">
					<button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="prevPage">上一页</button>
					<button class="btn btn-secondary btn-sm" :disabled="page * pageSize >= total" @click="nextPage">下一页</button>
				</div>
			</div>
		</div>
	</div>
</template>
