<script setup lang="ts">
import { ref, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NInput, NSelect, NDropdown } from 'naive-ui'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import DateTimeRangePicker from '@/components/common/DateTimeRangePicker.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import request from '@/utils/request'
import { tableScrollX } from '@/utils/renderUtils'
import { useExport } from '@/composables/useExport'

// timestamp(ms) ↔ 后端字符串 YYYY-MM-DD HH:mm:ss
function tsToStr(ts: number): string {
	const d = new Date(ts)
	const pad = (n: number) => String(n).padStart(2, '0')
	return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
// 默认查询当天：开始 = 当天 0 点，结束 = 当天 23:59:59
function todayStart(): string {
	const d = new Date()
	d.setHours(0, 0, 0, 0)
	return tsToStr(d.getTime())
}
function todayEnd(): string {
	const d = new Date()
	d.setHours(23, 59, 59, 999)
	return tsToStr(d.getTime())
}

const loading = ref(false)
const logs = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 查询表单数据
const filters = ref({
	start_date: todayStart(),
	end_date: todayEnd(),
	username: '',
	model: '',
	status: '',
	requestType: '',
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
	url: '/tenant/usage-logs/export',
	getFilters: () => {
		const params: any = {
			username: filters.value.username,
			model: filters.value.model,
			status: filters.value.status,
			request_type: filters.value.requestType,
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

// 详情弹窗
const detailModal = ref(false)
const detailLog = ref<any>(null)
const router = useRouter()

// 成本 Tooltip
const costTooltipVisible = ref(false)
const costTooltipPos = ref({ x: 0, y: 0 })
const costTooltipData = ref<any>(null)

// Token Tooltip
const tokenTooltipVisible = ref(false)
const tokenTooltipPos = ref({ x: 0, y: 0 })
const tokenTooltipData = ref<any>(null)

const statusBadgeClass: Record<string, string> = {
	success: 'bg-emerald-100 text-emerald-800',
	error: 'bg-red-100 text-red-800',
	interrupted: 'bg-amber-100 text-amber-800',
	timeout: 'bg-amber-100 text-amber-800',
	cancelled: 'bg-gray-100 text-gray-800',
}

const statusLabel: Record<string, string> = {
	success: '成功',
	error: '失败',
	interrupted: '中断',
	timeout: '超时',
	cancelled: '已取消',
}

const requestTypeBadge: Record<number, string> = {
	1: 'bg-gray-100 text-gray-800',
	2: 'bg-blue-100 text-blue-800',
	3: 'bg-orange-100 text-orange-800',
		4: 'bg-violet-100 text-violet-800',
}

const requestTypeLabel: Record<number, string> = {
	1: '同步',
	2: '流式',
	3: '异步',
		4: 'WebSocket',
}

const billingModeBadge: Record<string, string> = {
	token: 'bg-gray-100 text-gray-800',
	per_request: 'bg-blue-100 text-blue-800',
	tiered: 'bg-indigo-100 text-indigo-800',
}

const billingModeLabel: Record<string, string> = {
	token: '按量',
	per_request: '按次',
	tiered: '阶梯',
}

const billingSourceLabel: Record<string, string> = {
	base: '基础定价',
	tenant_custom: '租户独立价',
	tenant: '租户定价',
	custom: '自定义',
	plan: '套餐价',
	task: '异步任务',
}

async function fetchLogs() {
	loading.value = true
	try {
		const params: any = { page: page.value, page_size: pageSize.value }
		if (filters.value.username) params.username = filters.value.username
		if (filters.value.model) params.model = filters.value.model
		if (filters.value.status) params.status = filters.value.status
		if (filters.value.requestType) params.request_type = filters.value.requestType
		if (filters.value.start_date) params.start_date = filters.value.start_date
		if (filters.value.end_date) params.end_date = filters.value.end_date

		const res: any = await request.get('/tenant/usage-logs', { params })
		const raw = res.data?.data
		logs.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		logs.value = []
	} finally {
		loading.value = false
	}
}

function applyFilters() {
	page.value = 1
	fetchLogs()
}

function resetFilters() {
	filters.value = {
		start_date: todayStart(),
		end_date: todayEnd(),
		username: '',
		model: '',
		status: '',
		requestType: '',
	}
	page.value = 1
	fetchLogs()
}

function openDetail(log: any) {
	detailLog.value = log
	detailModal.value = true
}

function formatCost(n: any): string {
	const v = Number(n)
	if (n == null || isNaN(v)) return '$0.000000'
	return '$' + v.toFixed(6)
}

function formatMs(n: any): string {
	const v = Number(n)
	if (n == null || isNaN(v) || v <= 0) return '-'
	return v < 1000 ? `${v}ms` : `${(v / 1000).toFixed(2)}s`
}

function formatTime(s: string): string {
	if (!s) return '-'
	return s.replace('T', ' ').substring(0, 19)
}

function copyText(text: string) {
	navigator.clipboard.writeText(text).then(() => {}).catch(() => {})
}

function viewAuditLog(requestId: string, taskId?: string) {
	const query: Record<string, string> = {}
	if (taskId) query.task_id = taskId
	else query.request_id = requestId
	router.push({ name: 'TenantRequestAuditLogs', query })
}

function hasUpstreamModel(log: any): boolean {
	return log.upstream_model && log.upstream_model !== log.model_name && log.upstream_model !== ''
}

function parseSnapshot(log: any): any {
	if (!log.billing_snapshot) return null
	try {
		return typeof log.billing_snapshot === 'string' ? JSON.parse(log.billing_snapshot) : log.billing_snapshot
	} catch {
		return null
	}
}

// Tooltip helpers
function showCostTooltip(event: MouseEvent, row: any) {
	const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
	costTooltipData.value = row
	costTooltipPos.value = { x: rect.right + 8, y: rect.top + rect.height / 2 }
	costTooltipVisible.value = true
}

function hideCostTooltip() {
	costTooltipVisible.value = false
	costTooltipData.value = null
}

function showTokenTooltip(event: MouseEvent, row: any) {
	const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
	tokenTooltipData.value = row
	tokenTooltipPos.value = { x: rect.right + 8, y: rect.top + rect.height / 2 }
	tokenTooltipVisible.value = true
}

function hideTokenTooltip() {
	tokenTooltipVisible.value = false
	tokenTooltipData.value = null
}

// NDataTable 列定义
const columns = computed<DataTableColumns<any>>(() => [
	{
		title: '用户/项目',
		key: 'user',
		width: 160,
		render: (row) =>
			row.project_name
				? h('span', { class: 'text-sm text-primary-600 font-medium' }, row.project_name)
				: h('span', { class: 'text-sm text-gray-700' }, row.username || '-'),
	},
	{
		title: 'API Key',
		key: 'api_key',
		width: 160,
		render: (row) => h('span', { class: 'text-sm text-gray-700' }, row.api_key_name || row.api_key_id || '-'),
	},
	{
		title: '模型',
		key: 'model',
		width: 170,
		render: (row) =>
			hasUpstreamModel(row)
				? h('div', { class: 'space-y-0.5' }, [
						h('div', { class: 'font-medium text-gray-900 break-all' }, row.model_name),
						h('div', { class: 'text-gray-500 text-xs' }, [h('span', { class: 'mr-0.5' }, '↳'), row.upstream_model]),
				  ])
				: h('span', { class: 'font-medium text-gray-900' }, row.model_name),
	},
	{
		title: '类型',
		key: 'type',
		width: 120,
		render: (row) => {
			const children: any[] = [
				h(
					'span',
					{
						class: [
							'inline-flex items-center rounded px-2 py-0.5 text-xs font-medium',
							requestTypeBadge[row.request_type] || 'bg-gray-100 text-gray-800',
						],
					},
					requestTypeLabel[row.request_type] || '-'
				),
			]
			if (row.billing_mode) {
				children.push(
					h(
						'span',
						{
							class: [
								'ml-1 inline-flex items-center rounded px-2 py-0.5 text-xs font-medium',
								billingModeBadge[row.billing_mode] || 'bg-gray-100 text-gray-800',
							],
						},
						billingModeLabel[row.billing_mode] || row.billing_mode
					)
				)
			}
			return h('div', { class: 'flex items-center gap-1' }, children)
		},
	},
	{
		title: 'Token',
		key: 'token',
		width: 280,
		render: (row) =>
			h('div', { class: 'flex items-center gap-1.5' }, [
				h('div', { class: 'flex items-center gap-2' }, [
					h('div', { class: 'inline-flex items-center gap-1' }, [
						h(Icon, { name: 'arrowUp', size: 'sm', class: 'h-3.5 w-3.5 text-violet-500' }),
						h('span', { class: 'font-medium text-gray-900' }, (row.input_tokens || 0).toLocaleString()),
					]),
					h('div', { class: 'inline-flex items-center gap-1' }, [
						h(Icon, { name: 'arrowDown', size: 'sm', class: 'h-3.5 w-3.5 text-emerald-500' }),
						h('span', { class: 'font-medium text-gray-900' }, (row.output_tokens || 0).toLocaleString()),
					]),
					h('div', { class: 'inline-flex items-center gap-1' }, [
						h(Icon, { name: 'edit', size: 'xs', class: 'h-3.5 w-3.5 text-amber-500' }),
						h('span', { class: 'font-medium text-amber-600' }, (row.cache_creation_tokens || 0).toLocaleString()),
					]),
					h('div', { class: 'inline-flex items-center gap-1' }, [
						h(Icon, { name: 'bookOpen', size: 'xs', class: 'h-3.5 w-3.5 text-sky-500' }),
						h('span', { class: 'font-medium text-sky-600' }, (row.cache_read_tokens || 0).toLocaleString()),
					]),
				]),
				h(
					'div',
					{
						class: 'group relative',
						onClick: (e: Event) => e.stopPropagation(),
						onMouseenter: (e: MouseEvent) => showTokenTooltip(e, row),
						onMouseleave: hideTokenTooltip,
					},
					[
						h(
							'div',
							{ class: 'flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100' },
							[h(Icon, { name: 'infoCircle', size: 'xs', class: 'text-gray-400 group-hover:text-blue-500' })]
						),
					]
				),
			]),
	},
	{
		title: '费用',
		key: 'cost',
		width: 120,
		render: (row) =>
			h('div', { class: 'flex items-center gap-1.5' }, [
				h('span', { class: 'font-medium text-emerald-600' }, formatCost(row.actual_cost || row.total_cost)),
				h(
					'div',
					{
						class: 'group relative',
						onClick: (e: Event) => e.stopPropagation(),
						onMouseenter: (e: MouseEvent) => showCostTooltip(e, row),
						onMouseleave: hideCostTooltip,
					},
					[
						h(
							'div',
							{ class: 'flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100' },
							[h(Icon, { name: 'infoCircle', size: 'xs', class: 'text-gray-400 group-hover:text-blue-500' })]
						),
					]
				),
			]),
	},
	{
		title: '用时',
		key: 'latency',
		width: 110,
		render: (row) => {
			const children: any[] = [h('div', { class: 'text-sm text-gray-600' }, formatMs(row.latency_ms))]
			if (row.first_token_ms > 0) {
				children.push(h('div', { class: 'text-xs text-gray-400' }, 'TTFT ' + formatMs(row.first_token_ms)))
			}
			return h('div', { class: 'leading-tight' }, children)
		},
	},
	{
		title: '状态',
		key: 'status',
		width: 100,
		render: (row) => {
			const children: any[] = [
				h(
					'span',
					{
						class: [
							'inline-flex items-center rounded px-2 py-0.5 text-xs font-medium',
							statusBadgeClass[row.status] || 'bg-gray-100 text-gray-800',
						],
					},
					statusLabel[row.status] || row.status
				),
			]
			if (row.retry_index > 0) {
				children.push(
					h(
						'span',
						{
							class: 'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium leading-tight bg-amber-100 text-amber-600',
							title: '重试 ' + row.retry_index + ' 次',
						},
						'R' + row.retry_index
					)
				)
			}
			return h('div', { class: 'flex items-center gap-1' }, children)
		},
	},
	{
		title: '时间',
		key: 'created_at',
		width: 170,
		render: (row) =>
			h('span', { class: 'text-sm text-gray-600 whitespace-nowrap' }, formatTime(row.created_at)),
	},
	{
		title: '操作',
		key: 'actions',
		width: 60,
		fixed: 'right',
		align: 'right',
		render: (row) =>
			h(NButton, { size: 'small', onClick: () => openDetail(row) }, { icon: () => h(Icon, { name: 'eye', size: 'sm' }) }),
	},
])

// pageSize 变化回第 1 页并刷新
function handlePageSizeChange() {
	page.value = 1
	fetchLogs()
}

onMounted(() => {
	fetchLogs()
})
</script>

<template>
	<div class="viewport-table-page space-y-6">
		<!-- Filters -->
		<div class="card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="applyFilters">
					<DateTimeRangePicker
						v-model:start="filters.start_date"
						v-model:end="filters.end_date"
						@change="applyFilters"
					/>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
						<n-input v-model:value="filters.username" placeholder="搜索用户" style="width:120px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">模型名称</label>
						<n-input v-model:value="filters.model" placeholder="例如：gpt-4o" style="width:160px" @keydown.enter="applyFilters" />
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">状态</label>
						<n-select
							v-model:value="filters.status"
							:options="[
								{ value: '', label: '全部' },
								{ value: 'success', label: '成功' },
								{ value: 'error', label: '失败' },
								{ value: 'interrupted', label: '中断' },
								{ value: 'timeout', label: '超时' },
							]"
							style="width:100px"
						/>
					</div>
					<div class="flex items-center gap-2">
						<label class="text-sm text-gray-500 whitespace-nowrap">请求类型</label>
						<n-select
							v-model:value="filters.requestType"
							:options="[
								{ value: '', label: '全部' },
								{ value: '1', label: '同步' },
								{ value: '2', label: '流式' },
								{ value: '3', label: '异步' },
							]"
							style="width:100px"
						/>
					</div>
					<div class="ml-auto flex items-center gap-2">
						<button type="submit" class="btn btn-primary btn-sm">
							<Icon name="search" size="sm" />
							搜索
						</button>
						<button type="button" class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
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

		<!-- Logs Table / 移动端卡片 -->
		<div class="viewport-table-panel relative z-0 overflow-hidden">
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
				:data="logs"
				:row-key="(row: any) => row.id"
				card-title-key="model"
				card-badge-key="status"
				card-subtitle-key="created_at"
				:card-fields="[
					{ key: 'user' },
					{ key: 'api_key' },
					{ key: 'type' },
					{ key: 'token', full: true },
					{ key: 'cost' },
					{ key: 'latency' },
				]"
				card-actions-key="actions"
				:row-click="openDetail"
				@update:page="fetchLogs"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="document" size="xl" class="empty-state-icon" />
						<p class="empty-state-title">暂无用量日志</p>
						<p class="empty-state-description">日志将在 API 调用后展示</p>
					</div>
				</template>
			</ResponsiveDataTable>
		</div>

		<!-- Token Tooltip -->
		<Teleport to="body">
			<div
				v-if="tokenTooltipVisible && tokenTooltipData"
				class="fixed z-[9999] pointer-events-none -translate-y-1/2"
				:style="{ left: tokenTooltipPos.x + 'px', top: tokenTooltipPos.y + 'px' }"
			>
				<div class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl">
					<div class="space-y-1.5">
						<div>
							<div class="text-xs font-semibold text-gray-300 mb-1">Token 详情</div>
							<div class="flex items-center justify-between gap-4">
								<span class="text-gray-400">输入 Token</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.input_tokens || 0).toLocaleString() }}</span>
							</div>
							<div class="flex items-center justify-between gap-4">
								<span class="text-gray-400">输出 Token</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.output_tokens || 0).toLocaleString() }}</span>
							</div>
							<div  class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存创建</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.cache_creation_tokens || 0).toLocaleString() }}</span>
							</div>
							<div  class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存读取</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.cache_read_tokens || 0).toLocaleString() }}</span>
							</div>
							<div  class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存创建(5分钟)</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.cache_creation_5m_tokens || 0).toLocaleString() }}</span>
							</div>
							<div  class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存创建(1小时)</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.cache_creation_1h_tokens || 0).toLocaleString() }}</span>
							</div>
							<div v-if="tokenTooltipData.reasoning_tokens > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">推理 Token</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.reasoning_tokens || 0).toLocaleString() }}</span>
							</div>
							<div v-if="tokenTooltipData.audio_input_tokens > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">音频输入</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.audio_input_tokens || 0).toLocaleString() }}</span>
							</div>
							<div v-if="tokenTooltipData.audio_output_tokens > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">音频输出</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.audio_output_tokens || 0).toLocaleString() }}</span>
							</div>
							<div v-if="tokenTooltipData.image_output_tokens > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">图像输出</span>
								<span class="font-medium text-white">{{ (tokenTooltipData.image_output_tokens || 0).toLocaleString() }}</span>
							</div>
						</div>
						<div class="flex items-center justify-between gap-6 border-t border-gray-700 pt-1.5">
							<span class="text-gray-400">合计</span>
							<span class="font-semibold text-blue-400">{{ ((tokenTooltipData.input_tokens || 0) + (tokenTooltipData.output_tokens || 0) + (tokenTooltipData.cache_creation_tokens || 0) + (tokenTooltipData.cache_read_tokens || 0) + (tokenTooltipData.cache_creation_5m_tokens || 0) + (tokenTooltipData.cache_creation_1h_tokens || 0) + (tokenTooltipData.reasoning_tokens || 0) + (tokenTooltipData.audio_input_tokens || 0) + (tokenTooltipData.audio_output_tokens || 0) + (tokenTooltipData.image_output_tokens || 0)).toLocaleString() }}</span>
						</div>
					</div>
					<div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent" />
				</div>
			</div>
		</Teleport>

		<!-- Cost Tooltip -->
		<Teleport to="body">
			<div
				v-if="costTooltipVisible && costTooltipData"
				class="fixed z-[9999] pointer-events-none -translate-y-1/2"
				:style="{ left: costTooltipPos.x + 'px', top: costTooltipPos.y + 'px' }"
			>
				<div class="whitespace-nowrap rounded-lg border border-gray-700 bg-gray-900 px-3 py-2.5 text-xs text-white shadow-xl">
					<div class="space-y-1.5">
						<div class="mb-2 border-b border-gray-700 pb-1.5">
							<div class="text-xs font-semibold text-gray-300 mb-1">费用明细</div>
							<div class="flex items-center justify-between gap-4">
								<span class="text-gray-400">输入费用</span>
								<span class="font-medium text-white">{{ formatCost(costTooltipData.input_cost || 0) }}</span>
							</div>
							<div class="flex items-center justify-between gap-4">
								<span class="text-gray-400">输出费用</span>
								<span class="font-medium text-white">{{ formatCost(costTooltipData.output_cost || 0) }}</span>
							</div>
							<div v-if="costTooltipData.cache_creation_cost > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存创建费用</span>
								<span class="font-medium text-white">{{ formatCost(costTooltipData.cache_creation_cost) }}</span>
							</div>
							<div v-if="costTooltipData.cache_read_cost > 0" class="flex items-center justify-between gap-4">
								<span class="text-gray-400">缓存读取费用</span>
								<span class="font-medium text-white">{{ formatCost(costTooltipData.cache_read_cost) }}</span>
							</div>
						</div>
						<div v-if="costTooltipData.rate_multiplier && costTooltipData.rate_multiplier !== 1" class="flex items-center justify-between gap-6">
							<span class="text-gray-400">费率倍率</span>
							<span class="font-semibold text-blue-400">{{ Number(costTooltipData.rate_multiplier).toFixed(4) }}x</span>
						</div>
						<div class="flex items-center justify-between gap-6">
							<span class="text-gray-400">基础费用</span>
							<span class="font-medium text-white">{{ formatCost(costTooltipData.total_cost || 0) }}</span>
						</div>
						<div class="flex items-center justify-between gap-6">
							<span class="text-gray-400">实际费用</span>
							<span class="font-semibold text-emerald-400">{{ formatCost(costTooltipData.actual_cost || 0) }}</span>
						</div>
					</div>
					<div class="absolute right-full top-1/2 h-0 w-0 -translate-y-1/2 border-b-[6px] border-r-[6px] border-t-[6px] border-b-transparent border-r-gray-900 border-t-transparent" />
				</div>
			</div>
		</Teleport>

		<!-- Detail Modal -->
		<BaseModal
			:show="detailModal"
			title="用量详情"
			width="extra-wide"
			@close="detailModal = false"
		>
			<div v-if="detailLog" class="space-y-5">
				<!-- 基本信息 -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="document" size="sm" class="text-primary-500" />
						基本信息
					</h4>
					<div class="grid grid-cols-2 gap-x-6 gap-y-2.5 text-sm">
						<div class="flex justify-between">
							<span class="text-gray-500">请求 ID</span>
							<span class="font-mono text-xs flex items-center gap-1">
								{{ detailLog.request_id }}
								<button class="text-gray-400 hover:text-primary-500" @click.stop="copyText(detailLog.request_id)">
									<Icon name="copy" size="xs" />
								</button>
								<button class="text-xs text-primary-500 hover:text-primary-700 ml-1" @click.stop="viewAuditLog(detailLog.request_id, detailLog.task_id || undefined)">
									查看审计日志
								</button>
							</span>
						</div>
						<div v-if="detailLog.task_id" class="flex justify-between">
							<span class="text-gray-500">关联任务</span>
							<span class="font-mono text-xs flex items-center gap-1">
								{{ detailLog.task_id }}
								<button class="text-xs text-primary-500 hover:text-primary-700 ml-1" @click.stop="$router.push({ path: '/tenant/task-logs', query: { public_task_id: detailLog.task_id } })">查看任务</button>
							</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">代理模式</span>
							<span class="font-mono text-xs">{{ detailLog.relay_mode || '-' }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">请求类型</span>
							<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="requestTypeBadge[detailLog.request_type] || 'bg-gray-100 text-gray-800'">
								{{ requestTypeLabel[detailLog.request_type] || '-' }}
							</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">计费模式</span>
							<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="billingModeBadge[detailLog.billing_mode] || 'bg-gray-100 text-gray-800'">
								{{ billingModeLabel[detailLog.billing_mode] || '-' }}
							</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">状态</span>
							<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass[detailLog.status] || 'bg-gray-100 text-gray-800'">
								{{ statusLabel[detailLog.status] || detailLog.status }}
							</span>
						</div>
						<div v-if="detailLog.retry_index > 0" class="flex justify-between">
							<span class="text-gray-500">重试次数</span>
							<span class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium leading-tight bg-amber-100 text-amber-600">{{ detailLog.retry_index }}</span>
						</div>
						<div v-if="detailLog.api_key_id" class="flex justify-between">
							<span class="text-gray-500">API Key ID</span>
							<span class="font-mono text-xs">{{ detailLog.api_key_id }}</span>
						</div>
						<div v-if="detailLog.inbound_endpoint" class="flex justify-between">
							<span class="text-gray-500">请求端点</span>
							<span class="font-mono text-xs">{{ detailLog.inbound_endpoint }}</span>
						</div>
						<div v-if="detailLog.client_ip" class="flex justify-between">
							<span class="text-gray-500">客户端 IP</span>
							<span class="font-mono text-xs">{{ detailLog.client_ip }}</span>
						</div>
						<div v-if="detailLog.user_agent" class="col-span-2 flex justify-between">
							<span class="text-gray-500 shrink-0">User-Agent</span>
							<span class="text-xs text-gray-600 truncate ml-4 max-w-[360px]" :title="detailLog.user_agent">{{ detailLog.user_agent }}</span>
						</div>
						<div v-if="detailLog.service_tier" class="flex justify-between">
							<span class="text-gray-500">Service Tier</span>
							<span class="font-medium text-cyan-600">{{ detailLog.service_tier }}</span>
						</div>
						<div v-if="detailLog.reasoning_effort" class="flex justify-between">
							<span class="text-gray-500">Reasoning Effort</span>
							<span>{{ detailLog.reasoning_effort }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">时间</span>
							<span>{{ formatTime(detailLog.created_at) }}</span>
						</div>
					</div>
				</div>

				<!-- 模型映射 -->
				<div v-if="hasUpstreamModel(detailLog)">
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="arrowRight" size="sm" class="text-primary-500" />
						模型映射
					</h4>
					<div class="flex items-center gap-2 text-sm bg-gray-50 rounded-lg px-4 py-2.5">
						<span class="font-mono inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-800">{{ detailLog.model_name }}</span>
						<Icon name="arrowRight" size="sm" class="text-gray-400" />
						<span class="font-mono inline-flex items-center rounded px-2 py-0.5 text-xs font-medium bg-blue-100 text-blue-800">{{ detailLog.upstream_model }}</span>
					</div>
				</div>

				<!-- Token 使用量 -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="chart" size="sm" class="text-primary-500" />
						Token 使用量
					</h4>
					<div class="grid grid-cols-2 gap-2 text-sm">
						<div class="flex items-center justify-between rounded-lg bg-emerald-50 px-3 py-1.5">
							<span class="text-emerald-700">输入</span>
							<span class="font-semibold text-emerald-800 tabular-nums">{{ (detailLog.input_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg bg-violet-50 px-3 py-1.5">
							<span class="text-violet-700">输出</span>
							<span class="font-semibold text-violet-800 tabular-nums">{{ (detailLog.output_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg bg-sky-50 px-3 py-1.5">
							<span class="text-sky-700">缓存读取</span>
							<span class="font-semibold text-sky-800 tabular-nums">{{ (detailLog.cache_read_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg bg-amber-50 px-3 py-1.5">
							<span class="text-amber-700">缓存创建</span>
							<span class="font-semibold text-amber-800 tabular-nums">{{ (detailLog.cache_creation_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg bg-orange-50 px-3 py-1.5">
							<span class="text-orange-700">Cache 5m</span>
							<span class="font-semibold text-orange-800 tabular-nums">{{ (detailLog.cache_creation_5m_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="flex items-center justify-between rounded-lg bg-pink-50 px-3 py-1.5">
							<span class="text-pink-700">Cache 1h</span>
							<span class="font-semibold text-pink-800 tabular-nums">{{ (detailLog.cache_creation_1h_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.reasoning_tokens > 0" class="flex items-center justify-between rounded-lg bg-violet-50 px-3 py-1.5">
							<span class="text-violet-700">推理</span>
							<span class="font-semibold text-violet-800 tabular-nums">{{ (detailLog.reasoning_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.audio_input_tokens > 0" class="flex items-center justify-between rounded-lg bg-blue-50 px-3 py-1.5">
							<span class="text-blue-700">音频输入</span>
							<span class="font-semibold text-blue-800 tabular-nums">{{ (detailLog.audio_input_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.audio_output_tokens > 0" class="flex items-center justify-between rounded-lg bg-blue-50 px-3 py-1.5">
							<span class="text-blue-700">音频输出</span>
							<span class="font-semibold text-blue-800 tabular-nums">{{ (detailLog.audio_output_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.image_output_tokens > 0" class="flex items-center justify-between rounded-lg bg-red-50 px-3 py-1.5">
							<span class="text-red-700">图像输出</span>
							<span class="font-semibold text-red-800 tabular-nums">{{ (detailLog.image_output_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.image_count > 0" class="flex items-center justify-between rounded-lg bg-gray-50 px-3 py-1.5">
							<span class="text-gray-700">生成图片</span>
							<span class="font-semibold text-gray-800">{{ detailLog.image_count }} 张<span v-if="detailLog.image_size" class="text-xs text-gray-400 ml-1">{{ detailLog.image_size }}</span></span>
						</div>
					</div>
				</div>

				<!-- 费用明细 -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="creditCard" size="sm" class="text-primary-500" />
						费用明细
					</h4>
					<div class="space-y-2 text-sm">
						<div class="flex items-center justify-between">
							<span class="text-gray-500">输入费用</span>
							<span>{{ formatCost(detailLog.input_cost || 0) }}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-gray-500">输出费用</span>
							<span>{{ formatCost(detailLog.output_cost || 0) }}</span>
						</div>
						<div v-if="detailLog.cache_creation_cost > 0" class="flex items-center justify-between">
							<span class="text-gray-500">缓存创建费用</span>
							<span>{{ formatCost(detailLog.cache_creation_cost) }}</span>
						</div>
						<div v-if="detailLog.cache_read_cost > 0" class="flex items-center justify-between">
							<span class="text-gray-500">缓存读取费用</span>
							<span>{{ formatCost(detailLog.cache_read_cost) }}</span>
						</div>
						<div class="flex items-center justify-between border-t border-gray-200 pt-2 font-semibold">
							<span class="text-gray-700">实际费用</span>
							<span class="text-emerald-600">{{ formatCost(detailLog.actual_cost || 0) }}</span>
						</div>
						<div v-if="detailLog.plan_deduction > 0 || detailLog.wallet_deduction > 0" class="flex items-center justify-between">
							<span class="text-gray-500">扣费来源</span>
							<span class="flex items-center gap-1.5">
								<span v-if="detailLog.plan_deduction > 0" class="inline-flex items-center rounded-full bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-700">套餐 ${{ formatCost(detailLog.plan_deduction) }}</span>
								<span v-if="detailLog.wallet_deduction > 0" class="inline-flex items-center rounded-full bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700">余额 ${{ formatCost(detailLog.wallet_deduction) }}</span>
							</span>
						</div>
						<div v-if="detailLog.rate_multiplier && detailLog.rate_multiplier !== 1" class="flex items-center justify-between">
							<span class="text-gray-500">费率倍率</span>
							<span class="font-medium" :class="Number(detailLog.rate_multiplier) < 1 ? 'text-emerald-600' : 'text-amber-600'">{{ Number(detailLog.rate_multiplier).toFixed(4) }}x</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-gray-500">定价来源</span>
							<span>{{ billingSourceLabel[detailLog.billing_source] || detailLog.billing_source || '-' }}</span>
						</div>
						<div class="flex items-center justify-between">
							<span class="text-gray-500">货币</span>
							<span>{{ detailLog.currency || 'USD' }}</span>
						</div>
					</div>
				</div>

				<!-- 结算明细 -->
				<div v-if="detailLog.pre_deduct_amount > 0 || detailLog.refund_amount > 0 || detailLog.supplement_amount > 0 || detailLog.settled_at">
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="refresh" size="sm" class="text-primary-500" />
						结算明细
					</h4>
					<div class="space-y-2 text-sm">
						<div class="flex items-center justify-between">
							<span class="text-gray-500">预扣金额</span>
							<span>{{ formatCost(detailLog.pre_deduct_amount) }}</span>
						</div>
						<div v-if="detailLog.refund_amount > 0" class="flex items-center justify-between">
							<span class="text-gray-500">退回金额</span>
							<span class="text-emerald-600">{{ formatCost(detailLog.refund_amount) }}</span>
						</div>
						<div v-if="detailLog.supplement_amount > 0" class="flex items-center justify-between">
							<span class="text-gray-500">补扣金额</span>
							<span class="text-amber-600">{{ formatCost(detailLog.supplement_amount) }}</span>
						</div>
						<div v-if="detailLog.settled_at" class="flex items-center justify-between">
							<span class="text-gray-500">结算时间</span>
							<span>{{ formatTime(detailLog.settled_at) }}</span>
						</div>
					</div>
				</div>

				<!-- 性能指标 -->
				<div>
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="clock" size="sm" class="text-primary-500" />
						性能指标
					</h4>
					<div class="grid grid-cols-2 gap-x-6 gap-y-2 text-sm">
						<div class="flex justify-between">
							<span class="text-gray-500">总延迟</span>
							<span>{{ formatMs(detailLog.latency_ms) }}</span>
						</div>
						<div class="flex justify-between">
							<span class="text-gray-500">首 Token 延迟</span>
							<span>{{ formatMs(detailLog.first_token_ms) }}</span>
						</div>
						<div v-if="detailLog.stream_end_reason" class="flex justify-between">
							<span class="text-gray-500">流结束原因</span>
							<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="detailLog.stream_end_reason === 'done' ? 'bg-emerald-100 text-emerald-800' : 'bg-gray-100 text-gray-800'">
								{{ detailLog.stream_end_reason }}
							</span>
						</div>
					</div>
				</div>

				<!-- 计费快照：结构化 JSONB + 文本摘要 -->
				<div v-if="detailLog.billing_summary || parseSnapshot(detailLog)">
					<h4 class="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
						<Icon name="clipboard" size="sm" class="text-primary-500" />
						计费快照
					</h4>

					<!-- 结构化 JSONB 展示 -->
					<template v-if="parseSnapshot(detailLog)">
						<div class="grid grid-cols-2 gap-3">
							<!-- 定价信息 -->
							<div class="bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">定价信息</div>
								<div class="space-y-1 text-xs">
									<div v-if="parseSnapshot(detailLog).pricing.billing_source" class="flex justify-between">
										<span class="text-gray-500">价格来源</span>
										<span class="font-medium">{{ billingSourceLabel[parseSnapshot(detailLog).pricing.billing_source] || parseSnapshot(detailLog).pricing.billing_source }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.billing_mode" class="flex justify-between">
										<span class="text-gray-500">计费模式</span>
										<span class="font-medium">{{ billingModeLabel[parseSnapshot(detailLog).pricing.billing_mode] || parseSnapshot(detailLog).pricing.billing_mode }}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">基础输入价</span>
										<span class="font-mono">${{ Number(parseSnapshot(detailLog).pricing.base_input_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">基础输出价</span>
										<span class="font-mono">${{ Number(parseSnapshot(detailLog).pricing.base_output_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.effective_input_price !== parseSnapshot(detailLog).pricing.base_input_price" class="flex justify-between">
										<span class="text-gray-500">实际输入价</span>
										<span class="font-mono text-emerald-600">${{ Number(parseSnapshot(detailLog).pricing.effective_input_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.effective_output_price !== parseSnapshot(detailLog).pricing.base_output_price" class="flex justify-between">
										<span class="text-gray-500">实际输出价</span>
										<span class="font-mono text-emerald-600">${{ Number(parseSnapshot(detailLog).pricing.effective_output_price || 0).toFixed(6) }}/1M</span>
									</div>
								</div>
							</div>

							<!-- 倍率信息 -->
							<div class="bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">倍率信息</div>
								<div class="space-y-1 text-xs">
									<div class="flex justify-between">
										<span class="text-gray-500">模型倍率</span>
										<span class="font-mono">{{ Number(parseSnapshot(detailLog).multipliers.model_multiplier || 1).toFixed(4) }}x</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">租户倍率</span>
										<span class="font-mono" :class="Number(parseSnapshot(detailLog).multipliers.tenant_multiplier || 1) < 1 ? 'text-emerald-600' : ''">
											{{ Number(parseSnapshot(detailLog).multipliers.tenant_multiplier || 1).toFixed(4) }}x
										</span>
									</div>
									<div v-if="parseSnapshot(detailLog).multipliers.discount_ratio && parseSnapshot(detailLog).multipliers.discount_ratio !== 1" class="flex justify-between">
										<span class="text-gray-500">折扣比例</span>
										<span class="font-mono text-emerald-600">{{ Number(parseSnapshot(detailLog).multipliers.discount_ratio).toFixed(4) }}x</span>
									</div>
								</div>
							</div>

							<!-- Token 费用计算 -->
							<div v-if="parseSnapshot(detailLog).token_costs" class="col-span-2 bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">Token 费用计算</div>
								<div class="space-y-1 text-xs">
									<template v-for="(tc, key) in parseSnapshot(detailLog).token_costs" :key="key">
										<div v-if="(tc.tokens || 0) > 0" class="flex justify-between items-center">
											<span class="text-gray-500">{{ { input: '输入', output: '输出', cache_read: '缓存读取', cache_creation: '缓存创建', cache_creation_5m: '缓存创建(5分钟)', cache_creation_1h: '缓存创建(1小时)' }[key] || key }}</span>
											<span class="font-mono">
												{{ (tc.tokens || 0).toLocaleString() }} tokens &times; ${{ Number(tc.unit_price || 0).toFixed(6) }}/1M = <strong>${{ Number(tc.cost || 0).toFixed(6) }}</strong>
											</span>
										</div>
									</template>
								</div>
							</div>

							<!-- Cache 比率 -->
							<div v-if="parseSnapshot(detailLog).cache_ratios" class="bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">Cache 比率</div>
								<div class="space-y-1 text-xs">
									<div class="flex justify-between">
										<span class="text-gray-500">缓存读取比率</span>
										<span class="font-mono">{{ Number(parseSnapshot(detailLog).cache_ratios.cache_ratio || 0).toFixed(4) }}x</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">缓存创建比率</span>
										<span class="font-mono">{{ Number(parseSnapshot(detailLog).cache_ratios.cache_creation_ratio || 0).toFixed(4) }}x</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">5分钟缓存比率</span>
										<span class="font-mono">{{ Number(parseSnapshot(detailLog).cache_ratios.cache_creation_5m_ratio || 0).toFixed(4) }}x</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">1小时缓存比率</span>
										<span class="font-mono">{{ Number(parseSnapshot(detailLog).cache_ratios.cache_creation_1h_ratio || 0).toFixed(4) }}x</span>
									</div>
								</div>
							</div>

							<!-- 结算信息 -->
							<div v-if="parseSnapshot(detailLog).settlement" class="bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">结算信息</div>
								<div class="space-y-1 text-xs">
									<div class="flex justify-between">
										<span class="text-gray-500">预扣金额</span>
										<span class="font-mono">{{ formatCost(parseSnapshot(detailLog).settlement.pre_deduct_amount || 0) }}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">实际费用</span>
										<span class="font-mono text-emerald-600">{{ formatCost(parseSnapshot(detailLog).settlement.actual_cost || 0) }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).settlement.refund_amount > 0" class="flex justify-between">
										<span class="text-gray-500">退还金额</span>
										<span class="font-mono text-emerald-600">{{ formatCost(parseSnapshot(detailLog).settlement.refund_amount) }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).settlement.supplement_amount > 0" class="flex justify-between">
										<span class="text-gray-500">补扣金额</span>
										<span class="font-mono text-amber-600">{{ formatCost(parseSnapshot(detailLog).settlement.supplement_amount) }}</span>
									</div>
								</div>
							</div>

							<!-- 请求元信息 -->
							<div v-if="parseSnapshot(detailLog).request_meta" class="bg-gray-50 rounded-xl p-3">
								<div class="text-xs font-semibold text-gray-700 mb-2">请求元信息</div>
								<div class="space-y-1 text-xs">
									<div v-if="parseSnapshot(detailLog).request_meta.requested_model" class="flex justify-between">
										<span class="text-gray-500">请求模型</span>
										<span class="font-mono">{{ parseSnapshot(detailLog).request_meta.requested_model }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).request_meta.upstream_model" class="flex justify-between">
										<span class="text-gray-500">上游模型</span>
										<span class="font-mono">{{ parseSnapshot(detailLog).request_meta.upstream_model }}</span>
									</div>
									<div class="flex justify-between">
										<span class="text-gray-500">流式请求</span>
										<span>{{ parseSnapshot(detailLog).request_meta.is_stream ? '是' : '否' }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).request_meta.first_token_ms" class="flex justify-between">
										<span class="text-gray-500">首 Token</span>
										<span>{{ parseSnapshot(detailLog).request_meta.first_token_ms }}ms</span>
									</div>
								</div>
							</div>
						</div>
					</template>

					<!-- 文本摘要 -->
					<div v-if="detailLog.billing_summary" class="mt-3">
						<div class="text-xs font-semibold text-gray-500 mb-1">计算过程</div>
						<pre class="whitespace-pre-wrap rounded-lg bg-gray-900 px-4 py-3 text-xs leading-relaxed text-gray-100">{{ detailLog.billing_summary }}</pre>
					</div>
				</div>

				<!-- 错误信息 -->
				<div v-if="detailLog.error_message">
					<h4 class="text-sm font-semibold text-red-600 mb-3 flex items-center gap-2">
						<Icon name="xCircle" size="sm" />
						错误信息
					</h4>
					<div class="rounded-lg bg-red-50 border border-red-100 px-4 py-3 text-sm text-red-700 font-mono text-xs break-all">
						{{ detailLog.error_message }}
					</div>
				</div>
			</div>

			<template #footer>
				<button class="btn btn-secondary" @click="detailModal = false">关闭</button>
			</template>
		</BaseModal>
	</div>
</template>

