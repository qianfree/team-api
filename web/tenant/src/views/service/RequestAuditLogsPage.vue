<script setup lang="ts">
import { ref, reactive, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NButton, NInput } from 'naive-ui'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'
import DateTimeRangePicker from '@/components/common/DateTimeRangePicker.vue'

// 日期辅助（native，避免引入 dayjs 依赖）
function pad2(n: number): string {
	return String(n).padStart(2, '0')
}
// 默认查询当天：开始 = 当天 0 点，结束 = 当天 23:59:59
function todayStart(): string {
	const d = new Date()
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 00:00:00`
}
function todayEnd(): string {
	const d = new Date()
	return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())} 23:59:59`
}

interface RequestLog {
	id: number
	request_id: string
	method: string
	path: string
	status_code: number
	client_ip: string
	user_agent: string
	latency_ms: number
	first_token_ms: number
	audit_level: string
	task_id?: string
	task_status?: string
	task_completed_at?: string
	username?: string
	project_name?: string
	created_at: string
}

const logs = ref<RequestLog[]>([])
const logsLoading = ref(false)
const logPage = ref(1)
const logPageSize = ref(20)
const logTotal = ref(0)

const logFilter = reactive({
	username: '',
	request_id: '',
	task_id: '',
	path: '',
	status_code: '',
	start_date: todayStart(),
	end_date: todayEnd(),
})

const showDetail = ref(false)
const detailLoading = ref(false)
const detailRecord = ref<any>(null)

const auditLevelLabel: Record<string, string> = {
	full: '完整',
	masked: '脱敏',
	question_only: '仅提问',
	none: '关闭',
}

const taskStatusLabel: Record<string, string> = {
	SUBMITTED: '已提交',
	QUEUED: '排队中',
	IN_PROGRESS: '处理中',
	SUCCESS: '成功',
	FAILURE: '失败',
	TIMEOUT: '超时',
}

const taskStatusBadge: Record<string, string> = {
	SUBMITTED: 'bg-blue-100 text-blue-800',
	QUEUED: 'bg-cyan-100 text-cyan-800',
	IN_PROGRESS: 'bg-amber-100 text-amber-800',
	SUCCESS: 'bg-emerald-100 text-emerald-800',
	FAILURE: 'bg-red-100 text-red-800',
	TIMEOUT: 'bg-orange-100 text-orange-800',
}

function formatMs(ms: number): string {
	if (!ms && ms !== 0) return '-'
	if (ms < 1000) return `${ms}ms`
	return `${(ms / 1000).toFixed(2)}s`
}

function formatJson(str: string): string {
	if (!str) return ''
	try {
		return JSON.stringify(JSON.parse(str), null, 2)
	} catch {
		return str
	}
}

function parseUA(ua: string): string {
	if (!ua) return '-'
	let browser = ''
	let os = ''
	// Browser detection (order matters — Edg/OPR contain Chrome)
	if (ua.includes('OPR/') || ua.includes('Opera')) browser = 'Opera'
	else if (ua.includes('Edg/')) browser = 'Edge'
	else if (ua.includes('Firefox/')) browser = 'Firefox'
	else if (ua.includes('Chrome/')) browser = 'Chrome'
	else if (ua.includes('Safari/') && !ua.includes('Chrome')) browser = 'Safari'
	// Extract version
	const bv = ua.match(/(?:Chrome|Firefox|Edg|OPR|Safari)\/(\d+)/)
	if (bv) browser += ' ' + bv[1]
	// OS detection
	if (/iPhone|iPad/.test(ua)) { const m = ua.match(/(iPhone )?OS ([\d_]+)/); os = m ? 'iOS ' + m[2].replace(/_/g, '.') : 'iOS' }
	else if (ua.includes('Android')) { const m = ua.match(/Android ([\d.]+)/); os = m ? 'Android ' + m[1] : 'Android' }
	else if (ua.includes('Mac OS X')) { const m = ua.match(/Mac OS X ([\d_.]+)/); os = m ? 'macOS ' + m[1].replace(/_/g, '.') : 'macOS' }
	else if (ua.includes('Windows')) { const m = ua.match(/Windows NT ([\d.]+)/); os = m ? 'Win ' + ({ '10.0': '10/11', '6.3': '8.1', '6.2': '8', '6.1': '7' }[m[1]] || m[1]) : 'Windows' }
	else if (ua.includes('Linux')) os = 'Linux'
	if (!browser && !os) return ua.length > 30 ? ua.slice(0, 30) + '...' : ua
	if (browser && os) return browser + ' / ' + os
	return browser || os
}

function statusBadgeClass(code: number): string {
	if (code >= 200 && code < 300) return 'badge-success'
	if (code >= 400 && code < 500) return 'badge-warning'
	return 'badge-danger'
}

async function fetchRequestLogs() {
	logsLoading.value = true
	try {
		const params: Record<string, any> = {
			page: logPage.value,
			page_size: logPageSize.value,
		}
		if (logFilter.username) params.username = logFilter.username
		if (logFilter.request_id) params.request_id = logFilter.request_id
		if (logFilter.task_id) params.task_id = logFilter.task_id
		if (logFilter.path) params.path = logFilter.path
		if (logFilter.status_code) params.status_code = parseInt(logFilter.status_code)
		if (logFilter.start_date) params.start_date = logFilter.start_date
		if (logFilter.end_date) params.end_date = logFilter.end_date

		const res: any = await request.get('/tenant/audit/request-logs', { params })
		const raw = res.data?.data
		logs.value = Array.isArray(raw?.list) ? raw.list : []
		logTotal.value = raw?.total || 0
	} catch {
		logs.value = []
		logTotal.value = 0
	} finally {
		logsLoading.value = false
	}
}

async function fetchDetail(id: number) {
	showDetail.value = true
	detailLoading.value = true
	detailRecord.value = null
	try {
		const res: any = await request.get(`/tenant/audit/request-logs/${id}`)
		detailRecord.value = res.data?.data?.data || null
	} catch {
		detailRecord.value = null
	} finally {
		detailLoading.value = false
	}
}

function handleFilter() {
	logPage.value = 1
	fetchRequestLogs()
}

function handleReset() {
	logFilter.username = ''
	logFilter.request_id = ''
	logFilter.task_id = ''
	logFilter.path = ''
	logFilter.status_code = ''
	logFilter.start_date = todayStart()
	logFilter.end_date = todayEnd()
	logPage.value = 1
	fetchRequestLogs()
}

// NDataTable 列定义
const columns = computed<DataTableColumns<RequestLog>>(() => [
	{
		title: 'Request ID',
		key: 'request_id',
		render: (row) => h('span', { class: 'font-mono text-xs text-gray-500' }, row.request_id),
	},
	{
		title: '用户/项目',
		key: 'user',
		render: (row) =>
			row.project_name
				? h('span', { class: 'text-sm text-primary-600 font-medium' }, row.project_name)
				: h('span', { class: 'text-sm text-gray-700' }, row.username || '-'),
	},
	{
		title: '方法',
		key: 'method',
		render: (row) => h('span', { class: 'badge badge-gray text-xs' }, row.method),
	},
	{
		title: '路径',
		key: 'path',
		render: (row) => h('span', { class: 'font-mono text-xs text-gray-600' }, row.path),
	},
	{
		title: '状态码',
		key: 'status_code',
		render: (row) => h('span', { class: ['badge text-xs', statusBadgeClass(row.status_code)] }, String(row.status_code)),
	},
	{
		title: '客户端',
		key: 'user_agent',
		render: (row) => h('span', { class: 'text-xs text-gray-500', title: row.user_agent }, parseUA(row.user_agent)),
	},
	{
		title: '延迟',
		key: 'latency_ms',
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, formatMs(row.latency_ms)),
	},
	{
		title: '首Token',
		key: 'first_token_ms',
		render: (row) =>
			row.first_token_ms
				? h(
						'span',
						{
							class: [
								'text-xs',
								row.first_token_ms < 500
									? 'text-emerald-600'
									: row.first_token_ms < 1500
									? 'text-amber-600'
									: 'text-red-500',
							],
						},
						formatMs(row.first_token_ms)
				  )
				: h('span', { class: 'text-xs text-gray-300' }, '-'),
	},
	{
		title: '审计级别',
		key: 'audit_level',
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, auditLevelLabel[row.audit_level] || row.audit_level),
	},
	{
		title: '任务',
		key: 'task_id',
		render: (row) =>
			row.task_id
				? h(
						'span',
						{
							class: ['inline-flex items-center rounded px-2 py-0.5 text-xs font-medium', taskStatusBadge[row.task_status] || 'bg-gray-100 text-gray-800'],
						},
						taskStatusLabel[row.task_status] || row.task_status || '-'
				  )
				: h('span', { class: 'text-xs text-gray-300' }, '-'),
	},
	{
		title: '时间',
		key: 'created_at',
		render: (row) =>
			h('span', { class: 'text-xs text-gray-500' }, row.created_at ? new Date(row.created_at).toLocaleString() : '-'),
	},
	{
		title: '操作',
		key: 'actions',
		fixed: 'right',
		align: 'right',
		render: (row) =>
			h(
				NButton,
				{ text: true, type: 'primary', size: 'small', onClick: () => fetchDetail(row.id) },
				{ default: () => '详情' }
			),
	},
])

// pageSize 变化回第 1 页并刷新
function handlePageSizeChange() {
	logPage.value = 1
	fetchRequestLogs()
}

onMounted(() => {
	const route = useRoute()
	if (route.query.request_id) {
		logFilter.request_id = String(route.query.request_id)
	}
	if (route.query.task_id) {
		logFilter.task_id = String(route.query.task_id)
	}
	fetchRequestLogs()
})
</script>

<template>
	<div class="viewport-table-page space-y-6">
		<!-- Filter -->
		<div class="card">
			<div class="card-body !p-4">
				<form class="flex flex-wrap items-center gap-x-3 gap-y-3" @submit.prevent="handleFilter">
						<DateTimeRangePicker v-model:start="logFilter.start_date" v-model:end="logFilter.end_date" />
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">用户名</label>
							<n-input v-model:value="logFilter.username" placeholder="搜索用户" style="width:120px" @keydown.enter="handleFilter" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">Request ID</label>
							<n-input v-model:value="logFilter.request_id" placeholder="请求 ID" style="width:180px" @keydown.enter="handleFilter" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">Task ID</label>
							<n-input v-model:value="logFilter.task_id" placeholder="任务 ID" style="width:180px" @keydown.enter="handleFilter" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">请求路径</label>
							<n-input v-model:value="logFilter.path" placeholder="例如：/v1/chat" style="width:160px" @keydown.enter="handleFilter" />
						</div>
						<div class="flex items-center gap-2">
							<label class="text-sm text-gray-500 whitespace-nowrap">状态码</label>
							<n-input v-model:value="logFilter.status_code" placeholder="200" style="width:80px" @keydown.enter="handleFilter" />
						</div>
						<div class="ml-auto flex items-center gap-2">
							<button type="submit" class="btn btn-primary btn-sm">
								<Icon name="search" size="sm" />
								搜索
							</button>
							<button type="button" class="btn btn-secondary btn-sm" @click="handleReset">重置</button>
						</div>
					</form>
			</div>
		</div>
		<!-- Table -->
		<div class="viewport-table-panel card p-0 overflow-hidden">
			<n-data-table
				remote
				v-model:page="logPage"
				v-model:page-size="logPageSize"
				:item-count="logTotal"
				:page-sizes="[10, 20, 50, 100]"
				show-size-picker
				:loading="logsLoading"
				:columns="columns"
				:data="logs"
				:row-key="(row: RequestLog) => row.id"
				@update:page="fetchRequestLogs"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="clipboard" size="xl" class="empty-state-icon text-gray-300" />
						<p class="empty-state-title">暂无请求审计日志</p>
						<p class="empty-state-description">API 调用的输入输出记录将显示在这里</p>
					</div>
				</template>
			</n-data-table>
		</div>

		<!-- Detail Modal -->
		<Teleport to="body">
			<Transition name="modal">
				<div v-if="showDetail" class="modal-overlay" @click.self="showDetail = false">
					<div class="modal-content bg-white w-full max-w-2xl">
						<div class="modal-header">
							<h3 class="modal-title">请求详情</h3>
							<button class="btn btn-ghost btn-sm p-1" @click="showDetail = false">
								<Icon name="x" size="md" />
							</button>
						</div>
						<div class="modal-body">
							<!-- Loading -->
							<div v-if="detailLoading" class="p-8 text-center">
								<div class="spinner mx-auto mb-3"></div>
								<p class="text-sm text-gray-500">加载中...</p>
							</div>

							<template v-else-if="detailRecord">
								<!-- Meta info -->
								<div class="space-y-3 mb-6">
									<div class="grid grid-cols-2 gap-3 text-sm">
										<div>
											<span class="text-gray-500">Request ID</span>
											<p class="font-mono text-xs text-gray-700 mt-0.5 break-all">{{ detailRecord.request_id }}</p>
										</div>
										<div>
											<span class="text-gray-500">时间</span>
											<p class="text-gray-700 mt-0.5">{{ detailRecord.created_at }}</p>
										</div>
										<div>
											<span class="text-gray-500">方法 / 路径</span>
											<p class="font-mono text-xs text-gray-700 mt-0.5">{{ detailRecord.method }} {{ detailRecord.path }}</p>
										</div>
										<div>
											<span class="text-gray-500">状态码</span>
											<p class="mt-0.5">
												<span class="badge text-xs" :class="statusBadgeClass(detailRecord.status_code)">{{ detailRecord.status_code }}</span>
											</p>
										</div>
										<div>
											<span class="text-gray-500">延迟</span>
											<p class="text-gray-700 mt-0.5">{{ formatMs(detailRecord.latency_ms) }}</p>
										</div>
								<div>
									<span class="text-gray-500">首Token用时</span>
									<p v-if="detailRecord.first_token_ms" class="mt-0.5" :class="detailRecord.first_token_ms < 500 ? 'text-emerald-600' : detailRecord.first_token_ms < 1500 ? 'text-amber-600' : 'text-red-500'">{{ formatMs(detailRecord.first_token_ms) }}</p>
									<p v-else class="text-gray-300 mt-0.5">-</p>
								</div>
										<div>
											<span class="text-gray-500">客户端 IP</span>
											<p class="font-mono text-xs text-gray-700 mt-0.5">{{ detailRecord.client_ip }}</p>
										</div>
										<div>
											<span class="text-gray-500">审计级别</span>
											<p class="text-gray-700 mt-0.5">{{ auditLevelLabel[detailRecord.audit_level] || detailRecord.audit_level }}</p>
										</div>
										<div>
											<span class="text-gray-500">User-Agent</span>
											<p class="text-xs text-gray-700 mt-0.5 break-all">{{ detailRecord.user_agent || '-' }}</p>
										</div>
									</div>
								</div>

								<!-- Async Task Info -->
								<div v-if="detailRecord.task_id" class="mb-6">
									<h4 class="text-sm font-medium text-gray-700 mb-3 flex items-center gap-1.5">
										<svg class="h-4 w-4 text-primary-500" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
										异步任务
									</h4>
									<div class="bg-gray-50 rounded-xl p-4 space-y-2 text-sm">
										<div class="flex items-center justify-between">
											<span class="text-gray-500">任务 ID</span>
											<span class="font-mono text-xs">{{ detailRecord.task_id }}</span>
										</div>
										<div class="flex items-center justify-between">
											<span class="text-gray-500">任务状态</span>
											<span class="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" :class="taskStatusBadge[detailRecord.task_status] || 'bg-gray-100 text-gray-800'">{{ taskStatusLabel[detailRecord.task_status] || detailRecord.task_status || '-' }}</span>
										</div>
										<div v-if="detailRecord.task_completed_at" class="flex items-center justify-between">
											<span class="text-gray-500">完成时间</span>
											<span class="text-xs text-gray-700">{{ detailRecord.task_completed_at }}</span>
										</div>
									</div>
									<div v-if="detailRecord.task_result" class="mt-3">
										<h5 class="text-xs font-medium text-gray-500 mb-1">上游响应</h5>
										<pre class="code-block text-xs max-h-[300px] overflow-auto whitespace-pre-wrap break-all">{{ formatJson(detailRecord.task_result) }}</pre>
									</div>
								</div>

								<!-- Request Body -->
								<div v-if="detailRecord.request_body" class="mb-4">
									<h4 class="text-sm font-medium text-gray-700 mb-2">请求体（输入）</h4>
									<pre class="code-block text-xs max-h-[300px] overflow-auto whitespace-pre-wrap break-all">{{ formatJson(detailRecord.request_body) }}</pre>
								</div>

								<!-- Response Body -->
								<div v-if="detailRecord.response_body" class="mb-4">
									<h4 class="text-sm font-medium text-gray-700 mb-2">响应体（输出）</h4>
									<pre class="code-block text-xs max-h-[300px] overflow-auto whitespace-pre-wrap break-all">{{ formatJson(detailRecord.response_body) }}</pre>
								</div>

								<!-- No body hint -->
								<div v-if="!detailRecord.request_body && !detailRecord.response_body" class="p-4 bg-gray-50 rounded-xl text-sm text-gray-500 text-center">
									当前审计级别未记录请求/响应内容
								</div>
							</template>
						</div>
					</div>
				</div>
			</Transition>
		</Teleport>
	</div>
</template>

