<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import { useRoute } from 'vue-router'
import {
	Tag, Button,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({
	current: 1,
	pageSize: 20,
	total: 0,
	showPageSize: true,
	pageSizeOptions: [10, 20, 50],
})
const filter = reactive({
	tenant_id: '',
	api_key_id: '',
	username: '',
	request_id: '',
	method: '',
	status_code: '',
	date_range: [] as string[],
})

const detailLoading = ref(false)
const showDetail = ref(false)
const detailRecord = ref<any>(null)

const statusCodeColor: Record<number, string> = {
	200: 'green',
	400: 'orange',
	401: 'red',
	403: 'red',
	404: 'orange',
	429: 'red',
	500: 'red',
	502: 'red',
	503: 'red',
}

const auditLevelTagColor: Record<string, string> = {
	full: 'green',
	full_text: 'cyan',
	masked: 'arcoblue',
	question_only: 'orange',
	none: 'red',
}

const auditLevelLabel: Record<string, string> = {
	full: '完整审计',
	full_text: '全量文本',
	masked: '脱敏审计',
	question_only: '仅问答',
	none: '关闭',
}

function formatJson(str: string): string {
	if (!str) return ''
	try {
		return JSON.stringify(JSON.parse(str), null, 2)
	} catch {
		return str
	}
}

function parseJson(str: string): any {
	if (!str) return null
	try {
		return JSON.parse(str)
	} catch {
		return null
	}
}

function formatMs(ms: number): string {
	if (ms < 1000) return `${ms}ms`
	return `${(ms / 1000).toFixed(2)}s`
}

const columns: TableColumnData[] = [
	{ title: 'ID', dataIndex: 'id', width: 70 },
	{ title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true, render({ record }) {
			return record.tenant_name || record.tenant_id
		}},
	{ title: '用户/项目', dataIndex: 'username', width: 120, ellipsis: true, render({ record }) {
			if (record.project_name) return h('span', { style: 'color: #165dff' }, record.project_name)
			return record.username || '-'
		},
	},
	{ title: 'API Key', dataIndex: 'api_key_name', width: 120, ellipsis: true, render({ record }) {
			return record.api_key_name || record.api_key_id || '-'
		}},
	{ title: 'Request ID', dataIndex: 'request_id', width: 150, ellipsis: true },
	{
		title: '方法', dataIndex: 'method', width: 70,
		render({ record }) {
			const color = record.method === 'GET' ? 'arcoblue' : record.method === 'POST' ? 'green' : 'orange'
			return h(Tag, { color, size: 'small' }, () => record.method)
		},
	},
	{ title: '路径', dataIndex: 'path', width: 160, ellipsis: true },
	{
		title: '状态码', dataIndex: 'status_code', width: 80,
		render({ record }) {
			const color = statusCodeColor[record.status_code] || 'gray'
			return h(Tag, { color, size: 'small' }, () => String(record.status_code))
		},
	},
	{ title: '客户端IP', dataIndex: 'client_ip', width: 130 },
	{
		title: '用时', dataIndex: 'latency_ms', width: 90,
		render({ record }) {
			const ms = record.latency_ms
			const color = ms < 1000 ? 'green' : ms < 3000 ? 'orange' : 'red'
			return h(Tag, { color, size: 'small' }, () => formatMs(ms))
		},
	},
	{
		title: "首Token", dataIndex: "first_token_ms", width: 90,
		render({ record }) {
			const ms = record.first_token_ms
			if (!ms) return h("span", { style: "color: #c9cdd4" }, "-")
			const color = ms < 500 ? "green" : ms < 1500 ? "orange" : "red"
			return h(Tag, { color, size: "small" }, () => formatMs(ms))
		},
	},
	{
		title: '审计级别', dataIndex: 'audit_level', width: 90,
		render({ record }) {
			return h(Tag, { color: auditLevelTagColor[record.audit_level] || 'gray', size: 'small' }, () => auditLevelLabel[record.audit_level] || record.audit_level)
		},
	},
	{ title: '时间', dataIndex: 'created_at', width: 170 },
	{
		title: '', dataIndex: 'actions', width: 60, fixed: 'right',
		render({ record }) {
			return h(Button, {
				size: 'mini', type: 'text',
				onClick: () => fetchDetail(record.id),
			}, () => '详情')
		},
	},
]

async function fetchData() {
	loading.value = true
	try {
		const params: Record<string, any> = {
			page: pagination.current,
			page_size: pagination.pageSize,
		}
		if (filter.tenant_id) params.tenant_id = parseInt(filter.tenant_id)
		if (filter.api_key_id) params.api_key_id = parseInt(filter.api_key_id)
		if (filter.username) params.username = filter.username
		if (filter.method) params.method = filter.method
		if (filter.request_id) params.request_id = filter.request_id
		if (filter.status_code) params.status_code = parseInt(filter.status_code)
		if (filter.date_range.length === 2) {
			params.start_date = filter.date_range[0]
			params.end_date = filter.date_range[1]
		}
		const res: any = await request.get('/admin/audit/request-logs', { params })
		const raw = res.data?.data
		data.value = (Array.isArray(raw?.list) ? raw.list : []).filter(Boolean)
		pagination.total = Number(raw?.total) || 0
	} catch {
		data.value = []
		pagination.total = 0
	} finally {
		loading.value = false
	}
}

async function fetchDetail(id: number) {
	detailLoading.value = true
	showDetail.value = true
	try {
		const res: any = await request.get(`/admin/audit/request-logs/${id}`)
		const data = res.data?.data?.data || null
		if (data?.forwarding_trace) {
			data.forwarding_trace = parseJson(data.forwarding_trace)
		}
		detailRecord.value = data
	} catch {
		detailRecord.value = null
	} finally {
		detailLoading.value = false
	}
}

function handleFilter() {
	pagination.current = 1
	fetchData()
}

function handleReset() {
	filter.tenant_id = ''
	filter.api_key_id = ''
	filter.username = ''
	filter.request_id = ''
	filter.method = ''
	filter.status_code = ''
	filter.date_range = []
	pagination.current = 1
	fetchData()
}

onMounted(() => {
	const route = useRoute()
	if (route.query.request_id) {
		filter.request_id = String(route.query.request_id)
	}
	fetchData()
})
</script>

<template>
	<div class="page-table">
		<PageHeader title="请求日志" description="查看 LLM 请求审计日志，包含请求体和响应体" />

		<a-card :bordered="false" class="mb-4">
			<a-space wrap>
				<a-input
					v-model="filter.tenant_id"
					placeholder="租户ID"
					allow-clear
					style="width: 100px"
					@keydown.enter="handleFilter"
				/>
				<a-input
					v-model="filter.api_key_id"
					placeholder="Key ID"
					allow-clear
					style="width: 100px"
					@keydown.enter="handleFilter"
				/>
				<a-input
					v-model="filter.username"
					placeholder="用户名"
					allow-clear
					style="width: 120px"
					@keydown.enter="handleFilter"
				/>
				<a-input
					v-model="filter.request_id"
					placeholder="Request ID"
					allow-clear
					style="width: 200px"
					@keydown.enter="handleFilter"
				/>
				<a-select
					v-model="filter.method"
					placeholder="方法"
					allow-clear
					style="width: 100px"
				>
					<a-option value="GET">GET</a-option>
					<a-option value="POST">POST</a-option>
					<a-option value="PUT">PUT</a-option>
					<a-option value="DELETE">DELETE</a-option>
				</a-select>
				<a-input
					v-model="filter.status_code"
					placeholder="状态码"
					allow-clear
					style="width: 100px"
					@keydown.enter="handleFilter"
				/>
				<a-range-picker
					v-model="filter.date_range"
					style="width: 260px"
					format="YYYY-MM-DD"
				/>
				<a-button type="primary" @click="handleFilter">搜索</a-button>
				<a-button @click="handleReset">重置</a-button>
			</a-space>
		</a-card>

		<a-card :bordered="false">
			<a-table
				:columns="columns"
				:data="data"
				:loading="loading"
				:scroll="{ x: 1400 }"
				:bordered="false"
				:stripe="true"
				size="small"
				:pagination="false"
				row-key="id"
			/>
			<div class="table-footer">
				<a-pagination
					v-model:current="pagination.current"
					v-model:page-size="pagination.pageSize"
					:total="pagination.total"
					:page-size-options="pagination.pageSizeOptions"
					show-page-size
					@change="fetchData"
					@page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchData() }"
				/>
			</div>
		</a-card>

		<a-drawer v-model:visible="showDetail" :width="600" title="请求详情">
			<a-spin :loading="detailLoading" class="w-full">
				<template v-if="detailRecord">
					<a-descriptions :column="1" bordered size="medium">
						<a-descriptions-item label="Request ID">{{ detailRecord.request_id }}</a-descriptions-item>
						<a-descriptions-item label="租户/用户/Key">{{ detailRecord.tenant_id }} / {{ detailRecord.user_id }} / {{ detailRecord.api_key_id }}</a-descriptions-item>
						<a-descriptions-item label="方法">{{ detailRecord.method }}</a-descriptions-item>
						<a-descriptions-item label="路径">{{ detailRecord.path }}</a-descriptions-item>
						<a-descriptions-item v-if="detailRecord.query_params" label="查询参数">{{ detailRecord.query_params }}</a-descriptions-item>
						<a-descriptions-item label="状态码">{{ detailRecord.status_code }}</a-descriptions-item>
						<a-descriptions-item label="客户端IP">{{ detailRecord.client_ip }}</a-descriptions-item>
						<a-descriptions-item label="User-Agent">{{ detailRecord.user_agent }}</a-descriptions-item>
						<a-descriptions-item label="延迟">{{ formatMs(detailRecord.latency_ms) }}</a-descriptions-item>
					<a-descriptions-item label="首Token用时">{{ detailRecord.first_token_ms ? formatMs(detailRecord.first_token_ms) : '-' }}</a-descriptions-item>
						<a-descriptions-item label="审计级别">{{ auditLevelLabel[detailRecord.audit_level] || detailRecord.audit_level }}</a-descriptions-item>
						<a-descriptions-item label="时间">{{ detailRecord.created_at }}</a-descriptions-item>
					</a-descriptions>
					<div v-if="detailRecord.forwarding_trace" class="mt-4">
						<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">转发路径追踪</h4>
						<a-descriptions :column="1" bordered size="medium">
							<a-descriptions-item label="入口路径">{{ detailRecord.forwarding_trace.entry_path }}</a-descriptions-item>
							<a-descriptions-item label="入口格式">{{ detailRecord.forwarding_trace.entry_format }}</a-descriptions-item>
							<a-descriptions-item label="请求模型">{{ detailRecord.forwarding_trace.requested_model }}</a-descriptions-item>
							<a-descriptions-item label="上游模型">{{ detailRecord.forwarding_trace.upstream_model }}</a-descriptions-item>
							<a-descriptions-item v-if="detailRecord.forwarding_trace.model_mapped" label="模型映射">
								<Tag color="orange" size="small">是</Tag>
							</a-descriptions-item>
							<a-descriptions-item label="总尝试次数">{{ detailRecord.forwarding_trace.total_attempts }}</a-descriptions-item>
						</a-descriptions>
						<div v-if="detailRecord.forwarding_trace.hops?.length" style="margin-top:12px">
							<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">转发跳转</h4>
							<a-table
								:data="detailRecord.forwarding_trace.hops"
								:bordered="false"
								:stripe="true"
								size="mini"
								:pagination="false"
								:scroll="{ x: 700 }"
							>
								<template #columns>
									<a-table-column title="次数" data-index="attempt" :width="50" />
									<a-table-column title="渠道" :width="140">
										<template #cell="{ record }">
											{{ record.channel_name || '-' }} <span style="color:#86909c">#{{ record.channel_id }}</span>
										</template>
									</a-table-column>
									<a-table-column title="供应商" data-index="provider" :width="90" />
									<a-table-column title="上游模型" data-index="upstream_model" :width="130" ellipsis />
									<a-table-column title="状态" :width="65">
										<template #cell="{ record }">
											<Tag :color="record.success ? 'green' : 'red'" size="small">{{ record.success ? '成功' : '失败' }}</Tag>
										</template>
									</a-table-column>
									<a-table-column title="延迟" :width="80">
										<template #cell="{ record }">
											{{ record.latency_ms ? formatMs(record.latency_ms) : '-' }}
										</template>
									</a-table-column>
									<a-table-column title="错误" data-index="error" :width="160" ellipsis />
								</template>
							</a-table>
						</div>
					</div>
					<div v-if="detailRecord.request_body" class="mt-4">
						<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">请求体</h4>
						<pre class="audit-body">{{ formatJson(detailRecord.request_body) }}</pre>
					</div>
					<div v-if="detailRecord.response_body" class="mt-4">
						<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">响应体</h4>
						<pre class="audit-body">{{ formatJson(detailRecord.response_body) }}</pre>
					</div>
					<div v-if="detailRecord.request_headers" class="mt-4">
						<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">请求头</h4>
						<pre class="audit-body">{{ formatJson(detailRecord.request_headers) }}</pre>
					</div>
					<div v-if="detailRecord.response_headers" class="mt-4">
						<h4 style="margin-bottom:8px;color:var(--ta-text-primary)">响应头</h4>
						<pre class="audit-body">{{ formatJson(detailRecord.response_headers) }}</pre>
					</div>
				</template>
			</a-spin>
		</a-drawer>
	</div>
</template>

<style scoped>
.table-footer {
	display: flex;
	justify-content: flex-end;
	margin-top: 16px;
	padding-top: 16px;
	border-top: 1px solid var(--color-border-light, #e5e6eb);
}
.audit-body {
	background: var(--color-fill-2, #f7f8fa);
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	max-height: 400px;
	overflow: auto;
	white-space: pre-wrap;
	word-break: break-all;
	font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
}
</style>
