<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
	Message,
} from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'
import { useExport } from '@/composables/useExport'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
	current: 1,
	pageSize: 20,
	total: 0,
	showPageSize: true,
	pageSizeOptions: [10, 20, 50, 100],
})

const filterTenantId = ref<number | undefined>(undefined)
const filterUsername = ref('')
const filterModel = ref<string | undefined>(undefined)
const filterStatus = ref<string | undefined>(undefined)
const filterRequestType = ref<string | undefined>(undefined)
const filterDateRange = ref<string[]>([])

const tenantOptions = ref<{ label: string; value: number }[]>([])
const modelOptions = ref<{ label: string; value: string }[]>([])

const statusOptions = [
	{ label: '成功', value: 'success' },
	{ label: '失败', value: 'error' },
	{ label: '超时', value: 'timeout' },
	{ label: '已取消', value: 'cancelled' },
]

const requestTypeOptions = [
	{ label: '同步', value: '1' },
	{ label: '流式', value: '2' },
	{ label: '异步', value: '3' },
	{ label: 'WebSocket', value: '4' },
]

let tenantSearchTimer: ReturnType<typeof setTimeout> | null = null
let modelSearchTimer: ReturnType<typeof setTimeout> | null = null

async function fetchTenantOptions(keyword = '') {
	try {
		const res: any = await request.get('/admin/tenants/select', {
			params: { page: 1, page_size: 50, keyword }
		})
		const list = res.data?.data?.list || []
		tenantOptions.value = list.map((t: any) => ({
			label: `${t.name}（${t.code}）`,
			value: t.id,
		}))
	} catch {
		tenantOptions.value = []
	}
}

function handleTenantSearch(value: string) {
	if (tenantSearchTimer) clearTimeout(tenantSearchTimer)
	tenantSearchTimer = setTimeout(() => fetchTenantOptions(value), 300)
}

async function fetchModelOptions(search = '') {
	try {
		const res: any = await request.get('/admin/models', {
			params: { page: 1, page_size: 50, status: 'active', search }
		})
		const list = res.data?.data?.list || []
		modelOptions.value = list.map((m: any) => ({
			label: m.model_name,
			value: m.model_name,
		}))
	} catch {
		modelOptions.value = []
	}
}

function handleModelSearch(value: string) {
	if (modelSearchTimer) clearTimeout(modelSearchTimer)
	modelSearchTimer = setTimeout(() => fetchModelOptions(value), 300)
}

const statusTagColor: Record<string, string> = {
	success: 'green',
	failed: 'red',
	error: 'red',
	interrupted: 'orangered',
	timeout: 'orangered',
	cancelled: 'gray',
}

const statusLabel: Record<string, string> = {
	success: '成功',
	failed: '失败',
	error: '失败',
	interrupted: '中断',
	timeout: '超时',
	cancelled: '已取消',
}

const requestTypeLabel: Record<string, string> = {
	'1': '同步',
	'2': '流式',
	'3': '异步',
	'4': 'WebSocket',
}

const requestTypeColor: Record<string, string> = {
	'1': 'gray',
	'2': 'blue',
	'3': 'orange',
	'4': 'purple',
}

const billingModeLabel: Record<string, string> = {
	token: '按量',
	per_request: '按次',
	tiered: '阶梯',
}

const billingModeColor: Record<string, string> = {
	token: 'gray',
	per_request: 'blue',
	tiered: 'arcoblue',
}

const billingSourceLabel: Record<string, string> = {
	base: '基础定价',
	tenant_custom: '租户独立价',
	tenant: '租户定价',
	custom: '自定义',
	plan: '套餐价',
}

const detailVisible = ref(false)
const detailLog = ref<any>(null)
const router = useRouter()

function formatCost(n: number): string {
	if (n == null || isNaN(n)) return '$0.000000'
	return '$' + n.toFixed(6)
}

function formatMs(n: number): string {
	if (n == null || n <= 0) return '-'
	return n < 1000 ? `${n}ms` : `${(n / 1000).toFixed(2)}s`
}

function formatTime(s: string): string {
	if (!s) return '-'
	return s.replace('T', ' ').substring(0, 19)
}

function hasUpstreamModel(log: any): boolean {
	return log.upstream_model && log.upstream_model !== log.model_name && log.upstream_model !== ''
}

function totalTokens(log: any): number {
	return (log.input_tokens || 0) + (log.output_tokens || 0) +
		(log.cache_creation_tokens || 0) + (log.cache_read_tokens || 0) +
		(log.cache_creation_5m_tokens || 0) + (log.cache_creation_1h_tokens || 0) +
		(log.reasoning_tokens || 0) + (log.audio_input_tokens || 0) +
		(log.audio_output_tokens || 0) + (log.image_output_tokens || 0)
}

function parseSnapshot(log: any): any {
	if (!log.billing_snapshot) return null
	try {
		return typeof log.billing_snapshot === 'string' ? JSON.parse(log.billing_snapshot) : log.billing_snapshot
	} catch {
		return null
	}
}

function copyText(text: string) {
	navigator.clipboard.writeText(text).then(() => {
		Message.success('已复制')
	}).catch(() => {})
}

function viewAuditLog(requestId: string, taskId?: string) {
	const query: Record<string, string> = {}
	if (taskId) query.task_id = taskId
	else query.request_id = requestId
	router.push({ name: 'AdminRequestAuditLogs', query })
}

function openDetail(record: any) {
	detailLog.value = record
	detailVisible.value = true
}

async function fetchData() {
	loading.value = true
	try {
		const params: Record<string, any> = {
			page: pagination.current,
			page_size: pagination.pageSize,
		}
		if (filterTenantId.value) params.tenant_id = filterTenantId.value
		if (filterUsername.value) params.username = filterUsername.value
		if (filterModel.value) params.model = filterModel.value
		if (filterStatus.value) params.status = filterStatus.value
		if (filterRequestType.value) params.request_type = filterRequestType.value
		if (filterDateRange.value && filterDateRange.value.length === 2) {
			params.start_date = filterDateRange.value[0]
			params.end_date = filterDateRange.value[1]
		}

		const res: any = await request.get('/admin/usage-logs', { params })
		const raw = res.data?.data
		data.value = raw?.list || []
		pagination.total = raw?.total || 0
	} catch {
		data.value = []
		pagination.total = 0
	} finally {
		loading.value = false
	}
}

function handleFilter() {
	pagination.current = 1
	fetchData()
}

function handleReset() {
	filterTenantId.value = undefined
	filterUsername.value = ''
	filterModel.value = undefined
	filterStatus.value = undefined
	filterRequestType.value = undefined
	filterDateRange.value = []
	pagination.current = 1
	fetchData()
}

onMounted(() => {
	fetchTenantOptions()
	fetchModelOptions()
	fetchData()
})

const { exporting, exportFile } = useExport({
	url: '/admin/usage-logs/export',
	getFilters: () => ({
		tenant_id: filterTenantId.value,
		username: filterUsername.value,
		model: filterModel.value,
		status: filterStatus.value,
		request_type: filterRequestType.value,
		start_date: filterDateRange.value?.[0],
		end_date: filterDateRange.value?.[1],
	}),
})
</script>

<template>
	<div class="page-table">
		<PageHeader title="用量日志" description="查看所有租户的 API 调用记录和消费明细">
			<template #actions>
				<ADropdown trigger="hover">
					<AButton :loading="exporting">导出</AButton>
					<template #content>
						<ADoption @click="exportFile('csv')">导出 CSV</ADoption>
						<ADoption @click="exportFile('xlsx')">导出 Excel</ADoption>
					</template>
				</ADropdown>
				<a-button size="small" @click="handleReset">重置筛选</a-button>
			</template>
		</PageHeader>

		<a-card :bordered="false" class="mb-4">
			<a-space wrap>
				<a-select
						v-model="filterTenantId"
						:options="tenantOptions"
						placeholder="搜索租户"
						allow-search
						allow-clear
						:filter-option="false"
						style="width: 200px"
						@search="handleTenantSearch"
						@change="handleFilter"
						@clear="handleFilter"
					/>
				<a-input
					v-model="filterUsername"
					placeholder="用户名"
					allow-clear
					style="width: 120px"
					@keydown.enter="handleFilter"
				/>
				<a-select
						v-model="filterModel"
						:options="modelOptions"
						placeholder="搜索模型"
						allow-search
						allow-clear
						:filter-option="false"
						style="width: 200px"
						@search="handleModelSearch"
						@change="handleFilter"
						@clear="handleFilter"
					/>
				<a-select
					v-model="filterStatus"
					:options="statusOptions"
					placeholder="状态"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
				/>
				<a-select
					v-model="filterRequestType"
					:options="requestTypeOptions"
					placeholder="请求类型"
					allow-clear
					style="width: 120px"
					@change="handleFilter"
				/>
				<a-range-picker
					v-model="filterDateRange"
					style="width: 280px"
					@change="handleFilter"
				/>
				<a-button type="primary" @click="handleFilter">搜索</a-button>
			</a-space>
		</a-card>

		<a-card :bordered="false">
			<a-table
				:data="data"
				:loading="loading"
				:scroll="{ x: 1400 }"
				:bordered="false"
				:stripe="true"
				size="small"
				:pagination="false"
				row-key="id"
			>
				<template #columns>
					<a-table-column title="ID" data-index="id" :width="70" />
					<a-table-column title="租户" :width="120" :ellipsis="true">
						<template #cell="{ record }">
							{{ record.tenant_name || record.tenant_id }}
						</template>
					</a-table-column>
					<a-table-column title="用户/项目" :width="120" :ellipsis="true">
							<template #cell="{ record }">
								<span v-if="record.project_name" style="color: #165dff">
									{{ record.project_name }}
								</span>
								<span v-else>{{ record.username || '-' }}</span>
							</template>
						</a-table-column>
					<a-table-column title="API Key" :width="120" :ellipsis="true">
						<template #cell="{ record }">
							{{ record.api_key_name || record.api_key_id || '-' }}
						</template>
					</a-table-column>
					<a-table-column title="模型" data-index="model_name" :width="150">
						<template #cell="{ record }">
							<div v-if="hasUpstreamModel(record)">
								<div style="font-weight: 500">{{ record.model_name }}</div>
								<div class="upstream-model">↳ {{ record.upstream_model }}</div>
							</div>
							<span v-else style="font-weight: 500">{{ record.model_name }}</span>
						</template>
					</a-table-column>
					<a-table-column title="渠道" data-index="channel_name" :width="100" :ellipsis="true" />
					<a-table-column title="类型" data-index="request_type" :width="130">
						<template #cell="{ record }">
							<a-space :size="4">
								<a-tag :color="requestTypeColor[record.request_type]" size="small">
									{{ requestTypeLabel[record.request_type] || '-' }}
								</a-tag>
								<a-tag v-if="record.billing_mode" :color="billingModeColor[record.billing_mode]" size="small">
									{{ billingModeLabel[record.billing_mode] }}
								</a-tag>
							</a-space>
						</template>
					</a-table-column>
					<a-table-column title="Token" :width="200">
						<template #cell="{ record }">
							<a-tooltip background-color="#1e293b" :popup-style="{ padding: 0, borderRadius: '8px' }" position="right">
								<div class="token-cell">
									<div class="token-content">
										<div class="token-row">
											<span class="token-item token-in">↑{{ (record.input_tokens || 0).toLocaleString() }}</span>
											<span class="token-item token-out">↓{{ (record.output_tokens || 0).toLocaleString() }}</span>
											<span class="token-item token-cache-create">✎{{ (record.cache_creation_tokens || 0).toLocaleString() }}</span>
											<span class="token-item token-cache-read">⚡{{ (record.cache_read_tokens || 0).toLocaleString() }}</span>
										</div>
									</div>
									<span class="info-icon">i</span>
								</div>
								<template #content>
									<div class="dark-tooltip">
										<div class="dark-tooltip-title">Token 详情</div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">输入 Token</span><span class="dark-tooltip-value">{{ (record.input_tokens || 0).toLocaleString() }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">输出 Token</span><span class="dark-tooltip-value">{{ (record.output_tokens || 0).toLocaleString() }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">缓存创建</span><span class="dark-tooltip-value">{{ (record.cache_creation_tokens || 0).toLocaleString() }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">缓存读取</span><span class="dark-tooltip-value">{{ (record.cache_read_tokens || 0).toLocaleString() }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">缓存创建(5分钟)</span><span class="dark-tooltip-value">{{ (record.cache_creation_5m_tokens || 0).toLocaleString() }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">缓存创建(1小时)</span><span class="dark-tooltip-value">{{ (record.cache_creation_1h_tokens || 0).toLocaleString() }}</span></div>
										<div v-if="record.reasoning_tokens > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">推理 Token</span><span class="dark-tooltip-value">{{ record.reasoning_tokens.toLocaleString() }}</span></div>
										<div v-if="record.audio_input_tokens > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">音频输入</span><span class="dark-tooltip-value">{{ record.audio_input_tokens.toLocaleString() }}</span></div>
										<div v-if="record.audio_output_tokens > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">音频输出</span><span class="dark-tooltip-value">{{ record.audio_output_tokens.toLocaleString() }}</span></div>
										<div v-if="record.image_output_tokens > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">图像输出</span><span class="dark-tooltip-value">{{ record.image_output_tokens.toLocaleString() }}</span></div>
										<div class="dark-tooltip-divider" />
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">合计</span><span class="dark-tooltip-total">{{ totalTokens(record).toLocaleString() }}</span></div>
									</div>
								</template>
							</a-tooltip>
						</template>
					</a-table-column>
					<a-table-column title="费用" :width="140">
						<template #cell="{ record }">
							<a-tooltip background-color="#1e293b" :popup-style="{ padding: 0, borderRadius: '8px' }" position="right">
								<div class="cost-cell">
									<span class="cost-value">{{ formatCost(record.actual_cost || record.total_cost) }}</span>
									<span class="info-icon">i</span>
								</div>
								<template #content>
									<div class="dark-tooltip">
										<div class="dark-tooltip-title">费用明细</div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">输入费用</span><span class="dark-tooltip-value">{{ formatCost(record.input_cost || 0) }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">输出费用</span><span class="dark-tooltip-value">{{ formatCost(record.output_cost || 0) }}</span></div>
										<div v-if="record.cache_creation_cost > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">缓存创建费用</span><span class="dark-tooltip-value">{{ formatCost(record.cache_creation_cost) }}</span></div>
										<div v-if="record.cache_read_cost > 0" class="dark-tooltip-row"><span class="dark-tooltip-label">缓存读取费用</span><span class="dark-tooltip-value">{{ formatCost(record.cache_read_cost) }}</span></div>
										<div v-if="record.rate_multiplier && record.rate_multiplier !== 1" class="dark-tooltip-row"><span class="dark-tooltip-label">费率倍率</span><span class="dark-tooltip-highlight">{{ record.rate_multiplier.toFixed(4) }}x</span></div>
										<div class="dark-tooltip-divider" />
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">基础费用</span><span class="dark-tooltip-value">{{ formatCost(record.total_cost || 0) }}</span></div>
										<div class="dark-tooltip-row"><span class="dark-tooltip-label">实际费用</span><span class="dark-tooltip-success">{{ formatCost(record.actual_cost || 0) }}</span></div>
									</div>
								</template>
							</a-tooltip>
						</template>
					</a-table-column>
					<a-table-column title="延迟" data-index="latency_ms" :width="100">
						<template #cell="{ record }">
							<div style="line-height: 1.4">
								<div class="time-text">{{ formatMs(record.latency_ms) }}</div>
								<div v-if="record.first_token_ms > 0" class="sub-text" style="font-size: 11px">TTFT {{ formatMs(record.first_token_ms) }}</div>
							</div>
						</template>
					</a-table-column>
					<a-table-column title="状态" data-index="status" :width="90">
						<template #cell="{ record }">
							<a-space :size="4">
								<a-tag :color="statusTagColor[record.status]" size="small">
									{{ statusLabel[record.status] || record.status }}
								</a-tag>
								<span v-if="record.retry_index > 0" class="retry-badge">R{{ record.retry_index }}</span>
							</a-space>
						</template>
					</a-table-column>
					<a-table-column title="时间" data-index="created_at" :width="160">
						<template #cell="{ record }">
							<span class="time-text">{{ formatTime(record.created_at) }}</span>
						</template>
					</a-table-column>
					<a-table-column title="" :width="50">
						<template #cell="{ record }">
							<a-button type="text" size="mini" @click="openDetail(record)">详情</a-button>
						</template>
					</a-table-column>
				</template>
			</a-table>
			<div class="table-footer">
				<a-pagination
					v-model:current="pagination.current"
					v-model:page-size="pagination.pageSize"
					:total="pagination.total"
					:page-size-options="pagination.pageSizeOptions"
					show-page-size
					@change="fetchData"
					@page-size-change="(size: number) => { pagination.pageSize = size; pagination.current = 1; fetchData() }"
				/>
			</div>
		</a-card>

		<a-modal
			v-model:visible="detailVisible"
			title="用量详情"
			:width="720"
			:footer="false"
			unmount-on-close
		>
			<template v-if="detailLog">
				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-doc"></span>
						基本信息
					</div>
					<div class="detail-grid">
						<div class="detail-item">
							<span class="detail-label">请求 ID</span>
							<span class="detail-value mono-text">
								{{ detailLog.request_id }}
								<a-link class="copy-btn" @click="copyText(detailLog.request_id)">复制</a-link>
								<a-link class="copy-btn" @click="viewAuditLog(detailLog.request_id, detailLog.task_id || undefined)">查看审计日志</a-link>
							</span>
						</div>
						<div v-if="detailLog.task_id" class="detail-item">
							<span class="detail-label">关联任务</span>
							<span class="detail-value mono-text">
								{{ detailLog.task_id }}
								<a-link class="copy-btn" @click="$router.push({ path: '/admin/task-logs', query: { public_task_id: detailLog.task_id } })">查看任务</a-link>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">渠道</span>
							<span class="detail-value">
								{{ detailLog.channel_name || '-' }}
								<span v-if="detailLog.channel_type" class="sub-text">({{ detailLog.channel_type }})</span>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">代理模式</span>
							<span class="detail-value mono-text">{{ detailLog.relay_mode || '-' }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">请求类型</span>
							<span class="detail-value">
								<a-tag :color="requestTypeColor[detailLog.request_type]" size="small">
									{{ requestTypeLabel[detailLog.request_type] || '-' }}
								</a-tag>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">计费模式</span>
							<span class="detail-value">
								<a-tag :color="billingModeColor[detailLog.billing_mode]" size="small">
									{{ billingModeLabel[detailLog.billing_mode] || '-' }}
								</a-tag>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">状态</span>
							<span class="detail-value">
								<a-tag :color="statusTagColor[detailLog.status]" size="small">
									{{ statusLabel[detailLog.status] || detailLog.status }}
								</a-tag>
								<span v-if="detailLog.retry_index > 0" class="retry-badge">重试 {{ detailLog.retry_index }} 次</span>
							</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">API Key ID</span>
							<span class="detail-value mono-text">{{ detailLog.api_key_id || '-' }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">客户端 IP</span>
							<span class="detail-value mono-text">{{ detailLog.client_ip || '-' }}</span>
						</div>
						<div v-if="detailLog.inbound_endpoint" class="detail-item">
							<span class="detail-label">请求端点</span>
							<span class="detail-value mono-text">{{ detailLog.inbound_endpoint }}</span>
						</div>
						<div v-if="detailLog.service_tier" class="detail-item">
							<span class="detail-label">Service Tier</span>
							<span class="detail-value">{{ detailLog.service_tier }}</span>
						</div>
						<div v-if="detailLog.reasoning_effort" class="detail-item">
							<span class="detail-label">Reasoning Effort</span>
							<span class="detail-value">{{ detailLog.reasoning_effort }}</span>
						</div>
						<div v-if="detailLog.user_agent" class="detail-item detail-item-full">
							<span class="detail-label">User-Agent</span>
							<span class="detail-value sub-text" :title="detailLog.user_agent">{{ detailLog.user_agent }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">时间</span>
							<span class="detail-value">{{ formatTime(detailLog.created_at) }}</span>
						</div>
					</div>
				</div>

				<div v-if="hasUpstreamModel(detailLog)" class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-arrow"></span>
						模型映射
					</div>
					<div class="model-mapping">
						<a-tag>{{ detailLog.model_name }}</a-tag>
						<span class="mapping-arrow">&rarr;</span>
						<a-tag color="arcoblue">{{ detailLog.upstream_model }}</a-tag>
					</div>
				</div>

				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-chart"></span>
						Token 使用量
					</div>
					<div class="detail-grid">
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-in"></span> 输入 Token</span>
							<span class="detail-value">{{ (detailLog.input_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-out"></span> 输出 Token</span>
							<span class="detail-value">{{ (detailLog.output_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-cache-read"></span> 缓存读取</span>
							<span class="detail-value token-cache-read">{{ (detailLog.cache_read_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-cache-create"></span> 缓存创建</span>
							<span class="detail-value token-cache-create">{{ (detailLog.cache_creation_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-cache-5m"></span> 缓存创建(5分钟)</span>
							<span class="detail-value token-cache-5m">{{ (detailLog.cache_creation_5m_tokens || 0).toLocaleString() }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-cache-1h"></span> 缓存创建(1小时)</span>
							<span class="detail-value token-cache-1h">{{ (detailLog.cache_creation_1h_tokens || 0).toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.reasoning_tokens > 0" class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-reasoning"></span> 推理 Token</span>
							<span class="detail-value token-reasoning">{{ detailLog.reasoning_tokens.toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.audio_input_tokens > 0" class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-audio"></span> 音频输入</span>
							<span class="detail-value token-audio">{{ detailLog.audio_input_tokens.toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.audio_output_tokens > 0" class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-audio"></span> 音频输出</span>
							<span class="detail-value token-audio">{{ detailLog.audio_output_tokens.toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.image_output_tokens > 0" class="detail-item">
							<span class="detail-label"><span class="token-dot token-dot-image"></span> 图像输出</span>
							<span class="detail-value token-image">{{ detailLog.image_output_tokens.toLocaleString() }}</span>
						</div>
						<div v-if="detailLog.image_count > 0" class="detail-item">
							<span class="detail-label">生成图片</span>
							<span class="detail-value">
								{{ detailLog.image_count }} 张
								<span v-if="detailLog.image_size" class="sub-text">({{ detailLog.image_size }})</span>
							</span>
						</div>
					</div>
				</div>

				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-credit"></span>
						费用明细
					</div>
					<div class="detail-grid">
						<div class="detail-item">
							<span class="detail-label">输入费用</span>
							<span class="detail-value">{{ formatCost(detailLog.input_cost || 0) }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">输出费用</span>
							<span class="detail-value">{{ formatCost(detailLog.output_cost || 0) }}</span>
						</div>
						<div v-if="detailLog.cache_creation_cost > 0" class="detail-item">
							<span class="detail-label">缓存创建费用</span>
							<span class="detail-value">{{ formatCost(detailLog.cache_creation_cost) }}</span>
						</div>
						<div v-if="detailLog.cache_read_cost > 0" class="detail-item">
							<span class="detail-label">缓存读取费用</span>
							<span class="detail-value">{{ formatCost(detailLog.cache_read_cost) }}</span>
						</div>
					</div>
					<div class="detail-summary">
						<div class="detail-item">
							<span class="detail-label">费率倍率</span>
							<span
								v-if="detailLog.rate_multiplier && detailLog.rate_multiplier !== 1"
								class="detail-value"
								:class="detailLog.rate_multiplier < 1 ? 'text-success' : 'text-warning'"
							>
								{{ detailLog.rate_multiplier.toFixed(4) }}x
							</span>
							<span v-else class="detail-value">-</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">基础费用</span>
							<span class="detail-value">{{ formatCost(detailLog.total_cost || 0) }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">实际费用</span>
							<span class="detail-value text-success">{{ formatCost(detailLog.actual_cost || 0) }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">定价来源</span>
							<span class="detail-value">{{ billingSourceLabel[detailLog.billing_source] || detailLog.billing_source || '-' }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">货币</span>
							<span class="detail-value">{{ detailLog.currency || 'USD' }}</span>
						</div>
					</div>
				</div>

				<div v-if="detailLog.pre_deduct_amount > 0 || detailLog.refund_amount > 0 || detailLog.supplement_amount > 0" class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-refresh"></span>
						结算明细
					</div>
					<div class="detail-grid">
						<div class="detail-item">
							<span class="detail-label">预扣金额</span>
							<span class="detail-value">{{ formatCost(detailLog.pre_deduct_amount) }}</span>
						</div>
						<div v-if="detailLog.refund_amount > 0" class="detail-item">
							<span class="detail-label">退回金额</span>
							<span class="detail-value text-success">{{ formatCost(detailLog.refund_amount) }}</span>
						</div>
						<div v-if="detailLog.supplement_amount > 0" class="detail-item">
							<span class="detail-label">补扣金额</span>
							<span class="detail-value text-warning">{{ formatCost(detailLog.supplement_amount) }}</span>
						</div>
					</div>
				</div>

				<div class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-clock"></span>
						性能指标
					</div>
					<div class="detail-grid">
						<div class="detail-item">
							<span class="detail-label">总延迟</span>
							<span class="detail-value">{{ formatMs(detailLog.latency_ms) }}</span>
						</div>
						<div class="detail-item">
							<span class="detail-label">首 Token 延迟</span>
							<span class="detail-value">{{ formatMs(detailLog.first_token_ms) }}</span>
						</div>
						<div v-if="detailLog.stream_end_reason" class="detail-item">
							<span class="detail-label">流结束原因</span>
							<span class="detail-value">
								<a-tag
									:color="detailLog.stream_end_reason === 'done' ? 'green' : 'gray'"
									size="medium"
								>
									{{ detailLog.stream_end_reason }}
								</a-tag>
							</span>
						</div>
					</div>
				</div>

				<div v-if="detailLog.billing_summary || parseSnapshot(detailLog)" class="detail-section">
					<div class="detail-section-title">
						<span class="detail-icon detail-icon-clipboard"></span>
						计费快照
					</div>

					<template v-if="parseSnapshot(detailLog)">
						<div class="snapshot-grid">
							<div class="snapshot-block">
								<div class="snapshot-block-title">定价信息</div>
								<div class="snapshot-block-body">
									<div v-if="parseSnapshot(detailLog).pricing.billing_source" class="snapshot-row">
										<span class="snapshot-label">价格来源</span>
										<span class="snapshot-value">{{ billingSourceLabel[parseSnapshot(detailLog).pricing.billing_source] || parseSnapshot(detailLog).pricing.billing_source }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.billing_mode" class="snapshot-row">
										<span class="snapshot-label">计费模式</span>
										<span class="snapshot-value">{{ billingModeLabel[parseSnapshot(detailLog).pricing.billing_mode] || parseSnapshot(detailLog).pricing.billing_mode }}</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">基础输入价</span>
										<span class="snapshot-value">${{ (parseSnapshot(detailLog).pricing.base_input_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">基础输出价</span>
										<span class="snapshot-value">${{ (parseSnapshot(detailLog).pricing.base_output_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.effective_input_price !== parseSnapshot(detailLog).pricing.base_input_price" class="snapshot-row">
										<span class="snapshot-label">实际输入价</span>
										<span class="snapshot-value text-success">${{ (parseSnapshot(detailLog).pricing.effective_input_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).pricing.effective_output_price !== parseSnapshot(detailLog).pricing.base_output_price" class="snapshot-row">
										<span class="snapshot-label">实际输出价</span>
										<span class="snapshot-value text-success">${{ (parseSnapshot(detailLog).pricing.effective_output_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).cache_prices && parseSnapshot(detailLog).cache_prices.cache_creation_price > 0" class="snapshot-row">
										<span class="snapshot-label">缓存创建单价</span>
										<span class="snapshot-value">${{ (parseSnapshot(detailLog).cache_prices.cache_creation_price || 0).toFixed(6) }}/1M</span>
									</div>
									<div v-if="parseSnapshot(detailLog).cache_prices && parseSnapshot(detailLog).cache_prices.cache_read_price > 0" class="snapshot-row">
										<span class="snapshot-label">缓存读取单价</span>
										<span class="snapshot-value">${{ (parseSnapshot(detailLog).cache_prices.cache_read_price || 0).toFixed(6) }}/1M</span>
									</div>
								</div>
							</div>

							<div class="snapshot-block">
								<div class="snapshot-block-title">倍率信息</div>
								<div class="snapshot-block-body">
									<div class="snapshot-row">
										<span class="snapshot-label">模型倍率</span>
										<span class="snapshot-value">{{ (parseSnapshot(detailLog).multipliers.model_multiplier || 1).toFixed(4) }}x</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">租户倍率</span>
										<span class="snapshot-value" :class="(parseSnapshot(detailLog).multipliers.tenant_multiplier || 1) < 1 ? 'text-success' : ''">
											{{ (parseSnapshot(detailLog).multipliers.tenant_multiplier || 1).toFixed(4) }}x
										</span>
									</div>
									<div v-if="parseSnapshot(detailLog).multipliers.discount_ratio && parseSnapshot(detailLog).multipliers.discount_ratio !== 1" class="snapshot-row">
										<span class="snapshot-label">折扣比例</span>
										<span class="snapshot-value text-success">{{ (parseSnapshot(detailLog).multipliers.discount_ratio).toFixed(4) }}x</span>
									</div>
								</div>
							</div>

							<div v-if="parseSnapshot(detailLog).token_costs" class="snapshot-block snapshot-block-full">
								<div class="snapshot-block-title">Token 费用计算</div>
								<div class="snapshot-block-body">
									<div v-for="(tc, key) in parseSnapshot(detailLog).token_costs" :key="key" class="snapshot-row">
										<span class="snapshot-label">{{ { input: '输入', output: '输出', cache_read: '缓存读取', cache_creation: '缓存创建', cache_creation_5m: '缓存创建(5分钟)', cache_creation_1h: '缓存创建(1小时)' }[key] || key }}</span>
										<span class="snapshot-value">
											{{ (tc.tokens || 0).toLocaleString() }} tokens &times; ${{ (tc.unit_price || 0).toFixed(6) }}/1M = <strong>${{ (tc.cost || 0).toFixed(6) }}</strong>
										</span>
									</div>
								</div>
							</div>

							<div v-if="parseSnapshot(detailLog).settlement" class="snapshot-block">
								<div class="snapshot-block-title">结算信息</div>
								<div class="snapshot-block-body">
									<div class="snapshot-row">
										<span class="snapshot-label">预扣金额</span>
										<span class="snapshot-value">{{ formatCost(parseSnapshot(detailLog).settlement.pre_deduct_amount || 0) }}</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">实际费用</span>
										<span class="snapshot-value text-success">{{ formatCost(parseSnapshot(detailLog).settlement.actual_cost || 0) }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).settlement.refund_amount > 0" class="snapshot-row">
										<span class="snapshot-label">退还金额</span>
										<span class="snapshot-value text-success">{{ formatCost(parseSnapshot(detailLog).settlement.refund_amount) }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).settlement.supplement_amount > 0" class="snapshot-row">
										<span class="snapshot-label">补扣金额</span>
										<span class="snapshot-value text-warning">{{ formatCost(parseSnapshot(detailLog).settlement.supplement_amount) }}</span>
									</div>
								</div>
							</div>

							<div v-if="parseSnapshot(detailLog).request_meta" class="snapshot-block">
								<div class="snapshot-block-title">请求元信息</div>
								<div class="snapshot-block-body">
									<div v-if="parseSnapshot(detailLog).request_meta.requested_model" class="snapshot-row">
										<span class="snapshot-label">请求模型</span>
										<span class="snapshot-value mono-text">{{ parseSnapshot(detailLog).request_meta.requested_model }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).request_meta.upstream_model" class="snapshot-row">
										<span class="snapshot-label">上游模型</span>
										<span class="snapshot-value mono-text">{{ parseSnapshot(detailLog).request_meta.upstream_model }}</span>
									</div>
									<div class="snapshot-row">
										<span class="snapshot-label">流式请求</span>
										<span class="snapshot-value">{{ parseSnapshot(detailLog).request_meta.is_stream ? '是' : '否' }}</span>
									</div>
									<div v-if="parseSnapshot(detailLog).request_meta.first_token_ms" class="snapshot-row">
										<span class="snapshot-label">首 Token</span>
										<span class="snapshot-value">{{ parseSnapshot(detailLog).request_meta.first_token_ms }}ms</span>
									</div>
								</div>
							</div>
						</div>
					</template>

					<div v-if="detailLog.billing_summary" style="margin-top: 12px;">
						<div class="snapshot-text-title">计算过程</div>
						<pre class="billing-snapshot">{{ detailLog.billing_summary }}</pre>
					</div>
				</div>

				<div v-if="detailLog.error_message" class="detail-section">
					<div class="detail-section-title error-title">
						<span class="detail-icon detail-icon-error"></span>
						错误信息
					</div>
					<div class="error-message">{{ detailLog.error_message }}</div>
				</div>
			</template>
		</a-modal>
	</div>
</template>

<style scoped>
/* Token 单元格 */
.token-cell {
	display: flex;
	align-items: center;
	gap: 6px;
}
.token-content {
	min-width: 0;
}
.token-row {
	display: flex;
	align-items: center;
	gap: 12px;
	justify-content: space-between;
	line-height: 1.8;
	padding: 4px 0;
}
.token-item {
	font-size: 12px;
	font-weight: 600;
	white-space: nowrap;
	display: inline-flex;
	align-items: center;
	gap: 4px;
}
.token-in { color: #18a058; }
.token-out { color: #722ed1; }
.token-cache-read { color: #0fc6c2; }
.token-cache-create { color: #ff7d00; }
.token-cache-5m { color: #f77234; }
.token-cache-1h { color: #eb2f96; }
.token-reasoning { color: #7b61ff; }
.token-audio { color: #3491fa; }
.token-image { color: #f53f3f; }

/* 彩色圆点指示器 */
.token-dot {
	display: inline-block;
	width: 6px;
	height: 6px;
	border-radius: 50%;
	flex-shrink: 0;
}
.token-dot-in { background: #18a058; }
.token-dot-out { background: #722ed1; }
.token-dot-cache-read { background: #0fc6c2; }
.token-dot-cache-create { background: #ff7d00; }
.token-dot-cache-5m { background: #f77234; }
.token-dot-cache-1h { background: #eb2f96; }
.token-dot-reasoning { background: #7b61ff; }
.token-dot-audio { background: #3491fa; }
.token-dot-image { background: #f53f3f; }

/* Tooltip 触发图标 */
.info-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 16px;
	height: 16px;
	min-width: 16px;
	border-radius: 50%;
	background: #f2f3f5;
	color: #86909c;
	font-size: 11px;
	font-style: italic;
	font-weight: 600;
	cursor: help;
	flex-shrink: 0;
	transition: all 0.15s;
}
.info-icon:hover {
	background: #e8f3ff;
	color: #165dff;
}

/* 深色 Tooltip */
.dark-tooltip {
	min-width: 180px;
	font-size: 12px;
	background: #1e293b;
	color: #f1f5f9;
	padding: 10px 14px;
	border-radius: 8px;
	line-height: 1.8;
}
.dark-tooltip-title {
	font-weight: 600;
	color: #94a3b8;
	margin-bottom: 4px;
	font-size: 11px;
	text-transform: uppercase;
	letter-spacing: 0.5px;
}
.dark-tooltip-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: 20px;
	padding: 1px 0;
}
.dark-tooltip-label {
	color: #94a3b8;
}
.dark-tooltip-value {
	font-weight: 500;
	color: #f1f5f9;
}
.dark-tooltip-divider {
	height: 1px;
	background: #334155;
	margin: 6px 0;
}
.dark-tooltip-total {
	color: #38bdf8;
	font-weight: 700;
}
.dark-tooltip-highlight {
	color: #38bdf8;
	font-weight: 600;
}
.dark-tooltip-success {
	color: #34d399;
	font-weight: 700;
}

/* 费用单元格 */
.cost-cell {
	display: flex;
	align-items: center;
	gap: 6px;
}
.cost-value {
	font-weight: 600;
	color: #00b42a;
	font-size: 13px;
}

/* 模型上游 */
.upstream-model {
	font-size: 11px;
	color: #86909c;
}

/* 重试徽章 */
.retry-badge {
	display: inline-flex;
	align-items: center;
	padding: 1px 5px;
	font-size: 10px;
	font-weight: 600;
	line-height: 1.5;
	border-radius: 4px;
	background: #fff7e6;
	color: #d48806;
}

/* 时间 */
.time-text {
	font-size: 12px;
	color: #4e5969;
	white-space: nowrap;
}

/* 详情弹窗 */
.detail-section {
	margin-top: 16px;
}
.detail-section:first-child {
	margin-top: 0;
}
.detail-section-title {
	font-size: 13px;
	font-weight: 600;
	color: #1d2129;
	margin-bottom: 8px;
	padding-left: 8px;
	border-left: 3px solid #165dff;
	display: flex;
	align-items: center;
	gap: 6px;
}
.detail-icon {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	width: 16px;
	height: 16px;
}
.detail-icon-doc {
	content: '';
	width: 14px;
	height: 14px;
	border: 1.5px solid #165dff;
	border-radius: 2px;
}
.detail-icon-arrow {
	color: #165dff;
	font-size: 12px;
	font-weight: 700;
}
.detail-icon-chart {
	color: #165dff;
	font-size: 14px;
}
.detail-icon-credit {
	color: #165dff;
	font-size: 14px;
}
.detail-icon-refresh {
	color: #165dff;
	font-size: 13px;
}
.detail-icon-clock {
	color: #165dff;
	font-size: 14px;
}
.detail-icon-clipboard {
	color: #165dff;
	font-size: 13px;
}
.detail-icon-error {
	color: #f53f3f;
	font-size: 14px;
}
.detail-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 6px 24px;
}
.detail-item {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 4px 0;
	font-size: 13px;
}
.detail-item-full {
	grid-column: 1 / -1;
}
.detail-label {
	color: #86909c;
	flex-shrink: 0;
	display: flex;
	align-items: center;
	gap: 6px;
}
.detail-value {
	font-weight: 500;
	color: #1d2129;
	text-align: right;
	word-break: break-all;
}
.detail-summary {
	margin-top: 6px;
	padding-top: 6px;
	border-top: 1px solid #e5e6eb;
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 6px 24px;
}
.model-mapping {
	display: flex;
	align-items: center;
	gap: 8px;
	padding: 8px 12px;
	background: #f7f8fa;
	border-radius: 6px;
}
.mapping-arrow {
	color: #86909c;
	font-size: 14px;
}
.mono-text {
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
	font-size: 12px;
}
.sub-text {
	color: #86909c;
	font-size: 12px;
}
.copy-btn {
	margin-left: 6px;
	font-size: 12px;
}

/* 计费快照结构化展示 */
.snapshot-grid {
	display: grid;
	grid-template-columns: 1fr 1fr;
	gap: 10px;
}
.snapshot-block {
	background: #f7f8fa;
	border-radius: 6px;
	padding: 10px 12px;
}
.snapshot-block-full {
	grid-column: 1 / -1;
}
.snapshot-block-title {
	font-size: 12px;
	font-weight: 600;
	color: #1d2129;
	margin-bottom: 6px;
}
.snapshot-block-body {
	font-size: 12px;
}
.snapshot-row {
	display: flex;
	justify-content: space-between;
	align-items: center;
	padding: 2px 0;
	gap: 12px;
}
.snapshot-label {
	color: #86909c;
	white-space: nowrap;
}
.snapshot-value {
	font-weight: 500;
	color: #1d2129;
	text-align: right;
}
.snapshot-text-title {
	font-size: 12px;
	font-weight: 600;
	color: #86909c;
	margin-bottom: 4px;
}
.billing-snapshot {
	background: #1e293b;
	color: #c9d1d9;
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	font-family: 'SFMono-Regular', Consolas, monospace;
	white-space: pre-wrap;
	word-break: break-all;
	margin: 0;
	line-height: 1.6;
}
.error-title {
	border-left-color: #f53f3f;
}
.error-title .detail-section-title {
	color: #f53f3f;
}
.error-message {
	background: #fff2f0;
	border: 1px solid #ffccc7;
	color: #cb2634;
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	font-family: 'SFMono-Regular', Consolas, monospace;
	word-break: break-all;
}
.text-success { color: #00b42a !important; }
.text-warning { color: #ff7d00 !important; }

.table-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
	padding-top: 16px;
	border-top: 1px solid var(--color-border-light, #e5e6eb);
}
</style>
