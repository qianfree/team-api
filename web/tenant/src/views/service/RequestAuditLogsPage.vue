<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/common/Icon.vue'
import request from '@/utils/request'

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
	created_at: string
}

const logs = ref<RequestLog[]>([])
const logsLoading = ref(false)
const logPage = ref(1)
const logPageSize = 20
const logTotal = ref(0)
const logTotalPages = computed(() => Math.ceil(logTotal.value / logPageSize))

const logFilter = reactive({
	username: '',
	request_id: '',
	path: '',
	status_code: '',
	start_date: '',
	end_date: '',
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
			page_size: logPageSize,
		}
		if (logFilter.username) params.username = logFilter.username
		if (logFilter.request_id) params.request_id = logFilter.request_id
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

function handleLogPageChange(newPage: number) {
	logPage.value = newPage
	fetchRequestLogs()
}

function handleFilter() {
	logPage.value = 1
	fetchRequestLogs()
}

function handleReset() {
	logFilter.username = ''
	logFilter.request_id = ''
	logFilter.path = ''
	logFilter.status_code = ''
	logFilter.start_date = ''
	logFilter.end_date = ''
	logPage.value = 1
	fetchRequestLogs()
}

onMounted(() => {
	const route = useRoute()
	if (route.query.request_id) {
		logFilter.request_id = String(route.query.request_id)
	}
	fetchRequestLogs()
})
</script>

<template>
	<div class="space-y-6">
		<!-- Page Header -->
		<div class="page-header">
			<div>
				<h1 class="page-title">请求审计日志</h1>
				<p class="page-description">查看 API 请求的输入输出记录</p>
			</div>
		</div>

		<!-- Filter -->
		<div class="card">
			<div class="card-body">
				<div class="flex flex-wrap items-center gap-3">
					<input
						v-model="logFilter.username"
						class="input"
						placeholder="用户名"
						style="width: 120px"
						@keydown.enter="handleFilter"
					/>
					<input
						v-model="logFilter.request_id"
						class="input"
						placeholder="Request ID"
						style="width: 200px"
						@keydown.enter="handleFilter"
					/>
					<input
						v-model="logFilter.path"
						class="input"
						placeholder="请求路径"
						style="width: 180px"
						@keydown.enter="handleFilter"
					/>
					<input
						v-model="logFilter.status_code"
						class="input"
						placeholder="状态码"
						style="width: 100px"
						@keydown.enter="handleFilter"
					/>
					<input
						v-model="logFilter.start_date"
						type="date"
						class="input"
						style="width: 150px"
					/>
					<span class="text-gray-400 text-sm">至</span>
					<input
						v-model="logFilter.end_date"
						type="date"
						class="input"
						style="width: 150px"
					/>
					<button class="btn btn-primary btn-sm" @click="handleFilter">搜索</button>
					<button class="btn btn-secondary btn-sm" @click="handleReset">重置</button>
				</div>
			</div>
		</div>

		<!-- Table -->
		<div class="card p-0 overflow-hidden">
			<div class="card-header">
				<h2 class="text-lg font-semibold text-gray-900">请求记录</h2>
			</div>

			<!-- Loading -->
			<div v-if="logsLoading" class="p-8 text-center">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>

			<!-- Empty -->
			<div v-else-if="logs.length === 0" class="empty-state">
				<Icon name="clipboard" size="xl" class="empty-state-icon text-gray-300" />
				<p class="empty-state-title">暂无请求审计日志</p>
				<p class="empty-state-description">API 调用的输入输出记录将显示在这里</p>
			</div>

			<!-- Table -->
			<div v-else>
				<div class="table-container">
					<table class="table">
						<thead>
							<tr>
								<th>Request ID</th>
								<th>用户/项目</th>
								<th>方法</th>
								<th>路径</th>
								<th>状态码</th>
								<th>延迟</th>
							<th>首Token</th>
								<th>审计级别</th>
								<th>时间</th>
								<th></th>
							</tr>
						</thead>
						<tbody>
							<tr v-for="log in logs" :key="log.id">
								<td>
									<span class="font-mono text-xs text-gray-500 truncate max-w-[120px] inline-block">{{ log.request_id }}</span>
								</td>
								<td>
									<span v-if="log.project_name" class="text-sm text-primary-600 font-medium">{{ log.project_name }}</span>
										<span v-else class="text-sm text-gray-700">{{ log.username || '-' }}</span>
								</td>
								<td>
									<span class="badge badge-gray text-xs">{{ log.method }}</span>
								</td>
								<td>
									<span class="font-mono text-xs text-gray-600">{{ log.path }}</span>
								</td>
								<td>
									<span class="badge text-xs" :class="statusBadgeClass(log.status_code)">{{ log.status_code }}</span>
								</td>
								<td>
									<span class="text-xs text-gray-500">{{ formatMs(log.latency_ms) }}</span>
								</td>
							<td>
								<span v-if="log.first_token_ms" class="text-xs" :class="log.first_token_ms < 500 ? 'text-emerald-600' : log.first_token_ms < 1500 ? 'text-amber-600' : 'text-red-500'">{{ formatMs(log.first_token_ms) }}</span>
								<span v-else class="text-xs text-gray-300">-</span>
							</td>
								<td>
									<span class="text-xs text-gray-500">{{ auditLevelLabel[log.audit_level] || log.audit_level }}</span>
								</td>
								<td>
									<span class="text-xs text-gray-500">{{ log.created_at ? new Date(log.created_at).toLocaleString() : '-' }}</span>
								</td>
								<td>
									<button class="btn btn-ghost btn-sm text-primary-600" @click="fetchDetail(log.id)">详情</button>
								</td>
							</tr>
						</tbody>
					</table>
				</div>

				<!-- Pagination -->
				<div v-if="logTotalPages > 1" class="flex items-center justify-between px-6 py-3 border-t border-gray-100">
					<span class="text-xs text-gray-500">共 {{ logTotal }} 条记录</span>
					<div class="flex items-center gap-2">
						<button
							class="btn btn-ghost btn-sm"
							:disabled="logPage <= 1"
							@click="handleLogPageChange(logPage - 1)"
						>
							上一页
						</button>
						<span class="text-sm text-gray-600">{{ logPage }} / {{ logTotalPages }}</span>
						<button
							class="btn btn-ghost btn-sm"
							:disabled="logPage >= logTotalPages"
							@click="handleLogPageChange(logPage + 1)"
						>
							下一页
						</button>
					</div>
				</div>
			</div>
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
											<p class="text-xs text-gray-700 mt-0.5 truncate">{{ detailRecord.user_agent }}</p>
										</div>
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
