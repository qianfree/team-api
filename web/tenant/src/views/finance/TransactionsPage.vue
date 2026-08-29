<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput, NInputNumber, NSelect, NDropdown } from 'naive-ui'
import Icon from '@/components/common/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'
import { formatBilling } from '@/composables/useCurrency'

const loading = ref(false)
const transactions = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 日期格式化辅助函数
function formatDate(ts: number): string {
	const d = new Date(ts)
	return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}

// 查询表单数据
const filters = ref({
	type: '',
	start_date: '',
	end_date: '',
	amountMin: null as number | null,
	amountMax: null as number | null,
	username: '',
	model: '',
})

// 导出格式下拉选项（NDropdown 默认 teleport 到 body，避免被 .card 的 backdrop-filter
// stacking context 困住而被下方表格面板遮挡）
const exportDropdownOptions = [
	{ label: '导出 CSV', key: 'csv' },
	{ label: '导出 Excel', key: 'xlsx' },
]

function handleExport(format: string | number) {
	exportFile(format as 'csv' | 'xlsx')
}

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
		if (filters.value.start_date) {
			params.start_date = filters.value.start_date
		}
		if (filters.value.end_date) {
			params.end_date = filters.value.end_date
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

// 流水金额：bil 层数据直显本位币符号，showSign 为正数补 +、负数自带 -
function formatAmount(amount: number): string {
	return formatBilling(amount, 6, true)
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
		if (filters.value.start_date) params.start_date = filters.value.start_date
		if (filters.value.end_date) params.end_date = filters.value.end_date

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
		start_date: '',
		end_date: '',
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
		render: (row) => h('span', { class: 'text-gray-700' }, row.balance_after != null ? formatBilling(row.balance_after, 6) : '--'),
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
		<div class="card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="applyFilters">
					<DateRangePicker
						v-model:start="filters.start_date"
						v-model:end="filters.end_date"
						@change="applyFilters"
					/>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">类型</label>
						<n-select
							v-model:value="filters.type"
							:options="[
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
							]"
							style="width:120px"
						/>
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">最小金额</label>
						<n-input-number v-model:value="filters.amountMin" placeholder="最小" :min="0" :step="0.01" style="width:100px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">最大金额</label>
						<n-input-number v-model:value="filters.amountMax" placeholder="最大" :min="0" :step="0.01" style="width:100px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
						<n-input v-model:value="filters.username" placeholder="搜索用户" style="width:120px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">模型</label>
						<n-input v-model:value="filters.model" placeholder="例如：gpt-4o" style="width:150px" @keydown.enter="applyFilters" />
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button type="submit" class="btn btn-primary btn-sm">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
						<button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="fetchTransactions">
							<Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
							刷新
						</button>
						<span class="mx-1 h-6 w-px bg-gray-200" aria-hidden="true"></span>
						<n-dropdown trigger="click" :options="exportDropdownOptions" @select="handleExport">
							<button type="button" class="btn btn-secondary btn-sm" :disabled="exporting || loading">
								<Icon v-if="exporting" name="refresh" size="sm" class="animate-spin" />
								<Icon v-else name="download" size="sm" />
								导出
								<Icon name="chevronDown" size="xs" />
							</button>
						</n-dropdown>
					</div>
				</form>
			</div>
		</div>

		<!-- Transactions Table -->
		<div class="viewport-table-panel relative z-0">
			<ResponsiveDataTable
				remote
				fill-height
				v-model:page="page"
				v-model:page-size="pageSize"
				:item-count="total"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="loading"
				:columns="columns"
				:scroll-x="tableScrollX(columns)"
				:data="transactions"
				:row-key="(row: any) => row.id"
				card-title-key="amount"
				card-badge-key="type"
				card-subtitle-key="created_at"
				:card-fields="['balance_after', 'username', 'model_name', { key: 'request_id', full: true }, { key: 'description', full: true }]"
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
			</ResponsiveDataTable>
		</div>
	</div>
</template>
