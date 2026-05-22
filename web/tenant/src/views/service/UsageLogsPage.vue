<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import BaseModal from '@/components/common/BaseModal.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const logs = ref<any[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)

const filterUsername = ref('')
const filterModel = ref('')
const filterStatus = ref('')
const filterRequestType = ref('')
const filterStartDate = ref('')
const filterEndDate = ref('')

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/usage-logs/export',
	getFilters: () => ({
		username: filterUsername.value,
		model: filterModel.value,
		status: filterStatus.value,
		request_type: filterRequestType.value,
		start_date: filterStartDate.value,
		end_date: filterEndDate.value,
	}),
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
		const params: any = { page: page.value, page_size: pageSize }
		if (filterUsername.value) params.username = filterUsername.value
		if (filterModel.value) params.model = filterModel.value
		if (filterStatus.value) params.status = filterStatus.value
		if (filterRequestType.value) params.request_type = filterRequestType.value
		if (filterStartDate.value) params.start_date = filterStartDate.value
		if (filterEndDate.value) params.end_date = filterEndDate.value

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
	filterUsername.value = ''
	filterModel.value = ''
	filterStatus.value = ''
	filterRequestType.value = ''
	filterStartDate.value = ''
	filterEndDate.value = ''
	page.value = 1
	fetchLogs()
}

function prevPage() {
	if (page.value > 1) { page.value--; fetchLogs() }
}

function nextPage() {
	if (page.value * pageSize < total.value) { page.value++; fetchLogs() }
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

function hasCacheTokens(log: any): boolean {
	return (log.cache_creation_tokens > 0) || (log.cache_read_tokens > 0) ||
		(log.cache_creation_5m_tokens > 0) || (log.cache_creation_1h_tokens > 0)
}

function hasExtraTokens(log: any): boolean {
	return (log.reasoning_tokens > 0) || (log.audio_input_tokens > 0) ||
		(log.audio_output_tokens > 0) || (log.image_output_tokens > 0)
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

onMounted(() => {
	fetchLogs()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">用量日志</h1>
				<p class="page-description">查看 API 调用记录和消费明细</p>
			</div>
			<div class="relative inline-block">
				<button class="btn-secondary btn-sm inline-flex items-center gap-1.5" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
					<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
					<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
					导出
				</button>
				<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
					<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
					<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
				</div>
			</div>
		</div>

		<!-- Filters -->
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
							<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
							<input v-model="filterUsername" type="text" placeholder="搜索用户" class="input" style="width:120px" @keyup.enter="applyFilters" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">模型名称</label>
							<input v-model="filterModel" type="text" placeholder="例如：gpt-4o" class="input" style="width:160px" @keyup.enter="applyFilters" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">状态</label>
							<BaseSelect v-model="filterStatus" :options="[{value:'',label:'全部'},{value:'success',label:'成功'},{value:'error',label:'失败'},{value:'interrupted',label:'中断'},{value:'timeout',label:'超时'}]" container-class="w-[100px]" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">请求类型</label>
							<BaseSelect v-model="filterRequestType" :options="[{value:'',label:'全部'},{value:'1',label:'同步'},{value:'2',label:'流式'},{value:'3',label:'异步'}]" container-class="w-[100px]" />
						</div>
						<div class="ml-auto flex items-center gap-2">
							<button class="btn btn-primary btn-sm" @click="applyFilters">搜索</button>
							<button class="btn btn-secondary btn-sm" @click="resetFilters">重置</button>
						</div>
					</div>
				</div>
			</div>
		<!-- Logs Table -->
		<div class="card overflow-hidden">
			<div v-if="loading" class="p-8 flex justify-center">
				<div class="spinner h-6 w-6 border-primary-500"></div>
			</div>

			<div v-else-if="logs.length > 0" class="overflow-auto">
				<table class="table">
					<thead>
						<tr>
							<th class="min-w-50">用户/项目</th>
							<th class="min-w-40">API Key</th>
							<th class="min-w-45">模型</th>
							<th class="min-w-40">渠道</th>
							<th class="min-w-30">类型</th>
							<th class="min-w-30">Token</th>
							<th class="min-w-20">费用</th>
							<th class="min-w-30">用时</th>
							<th class="min-w-25">状态</th>
							<th class="min-w-35">时间</th>
							<th class="w-16"></th>
						</tr>
					</thead>
					<tbody>
						<tr v-for="log in logs" :key="log.id">
							<!-- 用户 -->
							<td>
								<span v-if="log.project_name" class="text-sm text-primary-600 font-medium">{{ log.project_name }}</span>
									<span v-else class="text-sm text-gray-700">{{ log.username || "-" }}</span>
							</td>

							<!-- API Key -->
								<td>
									<span class="text-sm text-gray-700">{{ log.api_key_name || log.api_key_id || '-' }}</span>
								</td>

								<!-- 模型 -->
							<td>
								<div v-if="hasUpstreamModel(log)" class="space-y-0.5">
									<div class="font-medium text-gray-900 break-all">{{ log.model_name }}</div>
									<div class="text-gray-500 text-xs"><span class="mr-0.5">↳</span>{{ log.upstream_model }}</div>
								</div>
								<span v-else class="font-medium text-gray-900">{{ log.model_name }}</span>
							</td>
							<!-- 渠道 -->
							<td>
								<span class="text-sm text-gray-700">{{ log.channel_name || '-' }}</span>
							</td>

							<!-- 请求类型 -->
							<td>
								<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="requestTypeBadge[log.request_type] || 'bg-gray-100 text-gray-800'">
									{{ requestTypeLabel[log.request_type] || '-' }}
								</span>
								<span v-if="log.billing_mode" class="ml-1 inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="billingModeBadge[log.billing_mode] || 'bg-gray-100 text-gray-800'">
									{{ billingModeLabel[log.billing_mode] || log.billing_mode }}
								</span>
							</td>

							<!-- Token -->
							<td>
								<div class="flex items-center gap-1.5">
									<div class="flex items-center gap-2">
										<div class="inline-flex items-center gap-1">
                      <Icon name="arrowUp" size="sm" class="h-3.5 w-3.5 text-violet-500" />
											<span class="font-medium text-gray-900">{{ (log.input_tokens || 0).toLocaleString() }}</span>
										</div>
										<div class="inline-flex items-center gap-1">
                      <Icon name="arrowDown" size="sm" class="h-3.5 w-3.5 text-emerald-500" />
											<span class="font-medium text-gray-900">{{ (log.output_tokens || 0).toLocaleString() }}</span>
										</div>
                    <div class="inline-flex items-center gap-1">
                      <Icon name="edit" size="xs" class="h-3.5 w-3.5 text-amber-500" />
                      <span class="font-medium text-amber-600">{{ (log.cache_creation_tokens || 0).toLocaleString() }}</span>
                    </div>
										<div class="inline-flex items-center gap-1">
											<Icon name="bookOpen" size="xs" class="h-3.5 w-3.5 text-sky-500" />
											<span class="font-medium text-sky-600">{{ (log.cache_read_tokens || 0).toLocaleString() }}</span>
										</div>
									</div>
									<!-- Token info tooltip trigger -->
									<div
										class="group relative"
										@click.stop
										@mouseenter="showTokenTooltip($event, log)"
										@mouseleave="hideTokenTooltip"
									>
										<div class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100">
											<Icon name="infoCircle" size="xs" class="text-gray-400 group-hover:text-blue-500" />
										</div>
									</div>
								</div>
							</td>

							<!-- 费用 -->
							<td>
								<div class="flex items-center gap-1.5">
									<span class="font-medium text-emerald-600">{{ formatCost(log.actual_cost || log.total_cost) }}</span>
									<div
										class="group relative"
										@click.stop
										@mouseenter="showCostTooltip($event, log)"
										@mouseleave="hideCostTooltip"
									>
										<div class="flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-gray-100 transition-colors group-hover:bg-blue-100">
											<Icon name="infoCircle" size="xs" class="text-gray-400 group-hover:text-blue-500" />
										</div>
									</div>
								</div>
							</td>

							<!-- 延迟 -->
							<td>
								<div class="leading-tight">
									<div class="text-sm text-gray-600">{{ formatMs(log.latency_ms) }}</div>
									<div v-if="log.first_token_ms > 0" class="text-xs text-gray-400">TTFT {{ formatMs(log.first_token_ms) }}</div>
								</div>
							</td>

							<!-- 状态 -->
							<td>
								<div class="flex items-center gap-1">
									<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass[log.status] || 'bg-gray-100 text-gray-800'">
										{{ statusLabel[log.status] || log.status }}
									</span>
									<span
										v-if="log.retry_index > 0"
										class="inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-medium leading-tight bg-amber-100 text-amber-600"
										:title="'重试 ' + log.retry_index + ' 次'"
									>R{{ log.retry_index }}</span>
								</div>
							</td>

							<!-- 时间 -->
							<td>
								<span class="text-sm text-gray-600 whitespace-nowrap">{{ formatTime(log.created_at) }}</span>
							</td>

							<!-- 详情按钮 -->
							<td>
								<button
									class="btn btn-ghost btn-sm p-1.5"
									title="查看详情"
									@click="openDetail(log)"
								>
									<Icon name="eye" size="sm" class="text-gray-400 hover:text-primary-500" />
								</button>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<div v-else class="empty-state">
				<Icon name="document" size="xl" class="empty-state-icon" />
				<p class="empty-state-title">暂无用量日志</p>
				<p class="empty-state-description">日志将在 API 调用后展示</p>
			</div>

			<!-- Pagination -->
			<div v-if="total > pageSize" class="px-6 py-4 border-t border-gray-100 flex items-center justify-between">
				<p class="text-sm text-gray-500">
					第 {{ page }} / {{ Math.ceil(total / pageSize) }} 页，共 {{ total }} 条
				</p>
				<div class="flex items-center gap-2">
					<button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="prevPage">上一页</button>
					<button class="btn btn-secondary btn-sm" :disabled="page * pageSize >= total" @click="nextPage">下一页</button>
				</div>
			</div>
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
							<span class="text-gray-500">渠道</span>
							<span>{{ detailLog.channel_name || '-' }} <span v-if="detailLog.channel_type" class="text-xs text-gray-400">(类型: {{ detailLog.channel_type }})</span></span>
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
									<div v-for="(tc, key) in parseSnapshot(detailLog).token_costs" :key="key" class="flex justify-between items-center">
										<span class="text-gray-500">{{ { input: '输入', output: '输出', cache_read: '缓存读取', cache_creation: '缓存创建', cache_creation_5m: '缓存创建(5分钟)', cache_creation_1h: '缓存创建(1小时)' }[key] || key }}</span>
										<span class="font-mono">
											{{ (tc.tokens || 0).toLocaleString() }} tokens &times; ${{ Number(tc.unit_price || 0).toFixed(6) }}/1M = <strong>${{ Number(tc.cost || 0).toFixed(6) }}</strong>
										</span>
									</div>
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
