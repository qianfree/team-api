<script setup lang="ts">
import { ref, onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '@/components/common/BaseSelect.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const transactions = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)

// Filter state
const filterType = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')
const filterAmountMin = ref('')
const filterAmountMax = ref('')
const filterUsername = ref('')
const filterModel = ref('')

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/wallet/transactions/export',
	getFilters: () => ({
		type: filterType.value,
		start_date: filterStartDate.value,
		end_date: filterEndDate.value,
		amount_min: filterAmountMin.value || undefined,
		amount_max: filterAmountMax.value || undefined,
		username: filterUsername.value,
		model_name: filterModel.value,
	}),
})

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

const typeOptions = [
	{ value: '', label: '全部' },
	{ value: 'recharge', label: '充值' },
	{ value: 'consume', label: '消费' },
	{ value: 'pre_deduct', label: '预扣' },
	{ value: 'settle', label: '结算' },
	{ value: 'refund', label: '退款' },
	{ value: 'adjust', label: '调整' },
	{ value: 'freeze', label: '冻结' },
	{ value: 'unfreeze', label: '解冻' },
]

function formatAmount(amount: number): string {
	if (amount >= 0) return '+$' + amount.toFixed(6)
	return '-$' + Math.abs(amount).toFixed(6)
}

async function fetchTransactions() {
	loading.value = true
	try {
		const params: any = { page: page.value, page_size: pageSize }
		if (filterType.value) params.type = filterType.value
		if (filterStartDate.value) params.start_date = filterStartDate.value
		if (filterEndDate.value) params.end_date = filterEndDate.value
		if (filterAmountMin.value) params.amount_min = filterAmountMin.value
		if (filterAmountMax.value) params.amount_max = filterAmountMax.value
		if (filterUsername.value) params.username = filterUsername.value
		if (filterModel.value) params.model_name = filterModel.value

		const res: any = await request.get('/tenant/wallet/transactions', {
			params,
		})
		const raw = res.data?.data
		transactions.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		transactions.value = []
	} finally {
		loading.value = false
	}
}

function applyFilters() {
	page.value = 1
	fetchTransactions()
}

function resetFilters() {
	filterType.value = ''
	filterStartDate.value = ''
	filterEndDate.value = ''
	filterAmountMin.value = ''
	filterAmountMax.value = ''
	filterUsername.value = ''
	filterModel.value = ''
	page.value = 1
	fetchTransactions()
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

onMounted(fetchTransactions)
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="flex items-start justify-between">
			<div>
				<h1 class="page-title">交易记录</h1>
				<p class="page-description">查看所有交易流水明细 · 共 {{ total }} 条记录</p>
			</div>
			<div class="flex items-center gap-2 flex-shrink-0">
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

		<!-- Filters -->
		<div class="card">
			<div class="card-body">
				<div class="flex flex-wrap items-center gap-4">
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">开始日期</label>
						<input v-model="filterStartDate" type="date" class="input" style="width:140px" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">结束日期</label>
						<input v-model="filterEndDate" type="date" class="input" style="width:140px" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">类型</label>
						<BaseSelect v-model="filterType" :options="typeOptions" container-class="w-[100px]" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">金额范围</label>
						<input v-model="filterAmountMin" type="number" step="0.01" placeholder="最小" class="input" style="width:100px" @keyup.enter="applyFilters" />
						<span class="text-gray-400">~</span>
						<input v-model="filterAmountMax" type="number" step="0.01" placeholder="最大" class="input" style="width:100px" @keyup.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
						<input v-model="filterUsername" type="text" placeholder="搜索用户" class="input" style="width:120px" @keyup.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">模型</label>
						<input v-model="filterModel" type="text" placeholder="例如：gpt-4o" class="input" style="width:150px" @keyup.enter="applyFilters" />
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button class="btn btn-primary btn-sm" @click="applyFilters">搜索</button>
						<button class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
					</div>
				</div>
			</div>
		</div>

		<!-- Transactions -->
		<div class="card">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="transactions.length > 0" class="table-container border-0 rounded-none">
				<table class="table">
					<thead>
						<tr>
							<th class="min-w-20">类型</th>
							<th class="min-w-30">金额</th>
							<th class="min-w-30">余额</th>
							<th class="min-w-35">用户</th>
							<th class="min-w-50">请求ID</th>
							<th class="min-w-40">模型</th>
							<th class="min-w-40">时间</th>
							<th class="min-w-200">描述</th>
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
							<td class="text-gray-700">${{ tx.balance_after?.toFixed(6) ?? '--' }}</td>
							<td class="text-gray-700 text-sm">{{ tx.username || '--' }}</td>
							<td class="text-gray-500 text-xs font-mono">{{ tx.request_id || '--' }}</td>
							<td class="text-gray-700 text-sm">{{ tx.model_name || '--' }}</td>
							<td class="text-gray-500 text-xs">{{ (tx.created_at || '').replace('T', ' ').substring(0, 16) }}</td>
							<td class="text-gray-500 text-sm">{{ tx.description || '--' }}</td>
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
