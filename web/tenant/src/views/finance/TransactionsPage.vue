<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput, NInputNumber } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '@/components/common/BaseSelect.vue'
import { renderBadge } from '@/utils/renderUtils'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const transactions = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// Filter state
const filterType = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')
const filterAmountMin = ref<number | null>(null)
const filterAmountMax = ref<number | null>(null)
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
	redemption: '兑换码',
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
	redemption: 'badge-success',
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
	{ value: 'redemption', label: '兑换码' },
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
		const params: any = { page: page.value, page_size: pageSize.value }
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
	filterAmountMin.value = null
	filterAmountMax.value = null
	filterUsername.value = ''
	filterModel.value = ''
	page.value = 1
	fetchTransactions()
}

// NDataTable 列定义
const columns = computed<DataTableColumns<any>>(() => [
	{ title: '类型', key: 'type', width: 80, render: (row) => renderBadge(row.type, txTypeLabel, txTypeBadgeClass) },
	{
		title: '金额',
		key: 'amount',
		width: 120,
		render: (row) =>
			h('span', { class: row.amount >= 0 ? 'text-emerald-600 font-semibold' : 'text-red-600 font-semibold' }, formatAmount(row.amount)),
	},
	{
		title: '余额',
		key: 'balance_after',
		width: 120,
		render: (row) => h('span', { class: 'text-gray-700' }, row.balance_after != null ? `$${row.balance_after.toFixed(6)}` : '--'),
	},
	{ title: '用户', key: 'username', width: 140, render: (row) => h('span', { class: 'text-gray-700 text-sm' }, row.username || '--') },
	{
		title: '请求ID',
		key: 'request_id',
		width: 200,
		render: (row) => h('span', { class: 'text-gray-500 text-xs font-mono' }, row.request_id || '--'),
	},
	{ title: '模型', key: 'model_name', width: 160, render: (row) => h('span', { class: 'text-gray-700 text-sm' }, row.model_name || '--') },
	{
		title: '时间',
		key: 'created_at',
		width: 160,
		render: (row) => h('span', { class: 'text-gray-500 text-xs' }, (row.created_at || '').replace('T', ' ').substring(0, 16)),
	},
	{
		title: '描述',
		key: 'description',
		width: 240,
		render: (row) => h('span', { class: 'text-gray-500 text-sm' }, row.description || '--'),
	},
])

function handlePageSizeChange() {
	page.value = 1
	fetchTransactions()
}

onMounted(fetchTransactions)
</script>

<template>
	<div class="viewport-table-page space-y-6">
		<!-- Filters -->
		<div class="relative z-20 overflow-visible card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="applyFilters">
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
						<n-input-number v-model:value="filterAmountMin" :min="0" :step="0.01" placeholder="最小" style="width:100px" @keyup.enter="applyFilters" />
						<span class="text-gray-400">~</span>
						<n-input-number v-model:value="filterAmountMax" :min="0" :step="0.01" placeholder="最大" style="width:100px" @keyup.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
						<n-input v-model:value="filterUsername" type="text" placeholder="搜索用户" style="width:120px" @keyup.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">模型</label>
						<n-input v-model:value="filterModel" type="text" placeholder="例如：gpt-4o" style="width:150px" @keyup.enter="applyFilters" />
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button type="submit" class="btn btn-primary btn-sm">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
						<span class="mx-1 h-6 w-px bg-gray-200" aria-hidden="true"></span>
						<button type="button" class="btn btn-ghost btn-sm" :disabled="loading" @click="fetchTransactions">
							<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
							刷新
						</button>
						<div class="relative">
							<button type="button" class="btn btn-secondary btn-sm" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
								<Icon v-if="exporting" name="refresh" size="sm" class="animate-spin" />
								<Icon v-else name="download" size="sm" />
								导出
								<Icon name="chevronDown" size="xs" />
							</button>
							<div v-if="showExportDropdown" class="absolute right-0 z-50 mt-2 w-36 overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg">
								<button type="button" class="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</button>
								<button type="button" class="block w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-50" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</button>
							</div>
						</div>
					</div>
				</form>
			</div>
		</div>

		<!-- Transactions -->
		<div class="viewport-table-panel relative z-0">
			<n-data-table
				remote
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:data="transactions"
				:row-key="(row: any) => row.id"
				@update:page="fetchTransactions"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="creditCard" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无交易记录</p>
						<p class="empty-state-description">交易记录将在 API 调用和充值后展示</p>
					</div>
				</template>
			</n-data-table>
		</div>
	</div>
</template>
