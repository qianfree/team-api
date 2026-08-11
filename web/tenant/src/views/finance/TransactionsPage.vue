<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import TableFilterForm, { type FilterField } from '@/components/common/TableFilterForm.vue'
import { renderBadge } from '@/utils/renderUtils'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const transactions = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 查询表单数据
const filters = ref({
	type: '',
	dateRange: null as [number, number] | null,
	amountMin: null as number | null,
	amountMax: null as number | null,
	username: '',
	model: '',
})

// 查询表单字段配置
const filterFields: FilterField[] = [
	{
		type: 'daterange',
		key: 'dateRange',
		label: '日期范围',
		width: '280px',
	},
	{
		type: 'select',
		key: 'type',
		label: '类型',
		width: '120px',
		options: [
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
		],
	},
	{
		type: 'number',
		key: 'amountMin',
		label: '最小金额',
		placeholder: '最小',
		width: '100px',
		min: 0,
		step: 0.01,
	},
	{
		type: 'number',
		key: 'amountMax',
		label: '最大金额',
		placeholder: '最大',
		width: '100px',
		min: 0,
		step: 0.01,
	},
	{
		type: 'input',
		key: 'username',
		label: '用户名',
		placeholder: '搜索用户',
		width: '120px',
	},
	{
		type: 'input',
		key: 'model',
		label: '模型',
		placeholder: '例如：gpt-4o',
		width: '150px',
	},
]

const { exporting, exportFile } = useExport({
	url: '/tenant/wallet/transactions/export',
	getFilters: () => {
		const params: any = {
			type: filters.value.type,
			amount_min: filters.value.amountMin || undefined,
			amount_max: filters.value.amountMax || undefined,
			username: filters.value.username,
			model_name: filters.value.model,
		}
		if (filters.value.dateRange) {
			const formatDate = (ts: number) => {
				const d = new Date(ts)
				return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
			}
			params.start_date = formatDate(filters.value.dateRange[0])
			params.end_date = formatDate(filters.value.dateRange[1])
		}
		return params
	},
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

function formatAmount(amount: number): string {
	if (amount >= 0) return '+$' + amount.toFixed(6)
	return '-$' + Math.abs(amount).toFixed(6)
}

async function fetchTransactions() {
	loading.value = true
	try {
		const params: any = { page: page.value, page_size: pageSize.value }
		if (filters.value.type) params.type = filters.value.type
		if (filters.value.amountMin) params.amount_min = filters.value.amountMin
		if (filters.value.amountMax) params.amount_max = filters.value.amountMax
		if (filters.value.username) params.username = filters.value.username
		if (filters.value.model) params.model_name = filters.value.model
		if (filters.value.dateRange) {
			const formatDate = (ts: number) => {
				const d = new Date(ts)
				return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
			}
			params.start_date = formatDate(filters.value.dateRange[0])
			params.end_date = formatDate(filters.value.dateRange[1])
		}

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
	filters.value = {
		type: '',
		dateRange: null,
		amountMin: null,
		amountMax: null,
		username: '',
		model: '',
	}
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
		<TableFilterForm
			v-model="filters"
			:fields="filterFields"
			:loading="loading"
			:show-export="true"
			:exporting="exporting"
			@search="applyFilters"
			@reset="resetFilters"
			@export="exportFile"
		>
			<template #extra-actions>
				<n-button :disabled="loading" @click="fetchTransactions">
					<template #icon>
						<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
					</template>
					刷新
				</n-button>
			</template>
		</TableFilterForm>

		<!-- Transactions Table -->
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
