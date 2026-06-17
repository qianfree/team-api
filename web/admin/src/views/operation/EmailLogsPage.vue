<script setup lang="ts">
import { ref, reactive, onMounted, watch, h } from 'vue'
import { Tag, Button } from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const activeTab = ref<'send-logs' | 'verify-codes'>('send-logs')

// ====== Tab 1: 邮件发送记录（ntf_send_log）======
const sendLoading = ref(false)
const sendData = ref<any[]>([])
const sendPagination = reactive({
	current: 1, pageSize: 20, total: 0,
	showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const sendFilter = reactive({
	status: '',
	recipient: '',
	date_range: [] as string[],
})

const statusColor: Record<string, string> = { sent: 'green', failed: 'red', pending: 'orange' }
const statusLabel: Record<string, string> = { sent: '已发送', failed: '发送失败', pending: '待发送' }

const sendColumns: TableColumnData[] = [
	{ title: 'ID', dataIndex: 'id', width: 80 },
	{ title: '收件人', dataIndex: 'recipient', width: 200, ellipsis: true },
	{ title: '主题', dataIndex: 'subject', width: 200, ellipsis: true },
	{ title: '模板', dataIndex: 'template_code', width: 120, ellipsis: true, render({ record }) { return record.template_code || '-' } },
	{
		title: '状态', dataIndex: 'status', width: 90,
		render({ record }) {
			return h(Tag, { color: statusColor[record.status] || 'gray', size: 'small' }, () => statusLabel[record.status] || record.status)
		},
	},
	{ title: '重试', dataIndex: 'retry_count', width: 60 },
	{
		title: '错误信息', dataIndex: 'error_message', width: 220, ellipsis: true,
		render({ record }) {
			return record.error_message ? h('span', { style: 'color:#f53f3f' }, record.error_message) : '-'
		},
	},
	{ title: '发送时间', dataIndex: 'sent_at', width: 170, render({ record }) { return record.sent_at || record.created_at } },
	{
		title: '操作', dataIndex: 'actions', width: 70, fixed: 'right',
		render({ record }) {
			return h(Button, { size: 'mini', type: 'text', onClick: () => fetchSendDetail(record.id) }, () => '详情')
		},
	},
]

async function fetchSendData() {
	sendLoading.value = true
	try {
		const params: Record<string, any> = { page: sendPagination.current, page_size: sendPagination.pageSize }
		if (sendFilter.status) params.status = sendFilter.status
		if (sendFilter.recipient) params.recipient = sendFilter.recipient
		if (sendFilter.date_range.length === 2) {
			params.start_date = sendFilter.date_range[0]
			params.end_date = sendFilter.date_range[1]
		}
		const res: any = await request.get('/admin/email/send-logs', { params })
		const raw = res.data?.data
		sendData.value = (Array.isArray(raw?.list) ? raw.list : []).filter(Boolean)
		sendPagination.total = Number(raw?.total) || 0
	} catch {
		sendData.value = []
		sendPagination.total = 0
	} finally {
		sendLoading.value = false
	}
}

function handleSendFilter() { sendPagination.current = 1; fetchSendData() }
function handleSendReset() {
	sendFilter.status = ''
	sendFilter.recipient = ''
	sendFilter.date_range = []
	sendPagination.current = 1
	fetchSendData()
}

// 详情抽屉
const detailLoading = ref(false)
const showDetail = ref(false)
const detailRecord = ref<any>(null)
async function fetchSendDetail(id: number) {
	detailLoading.value = true
	showDetail.value = true
	try {
		const res: any = await request.get(`/admin/email/send-logs/${id}`)
		detailRecord.value = res.data?.data || null
	} catch {
		detailRecord.value = null
	} finally {
		detailLoading.value = false
	}
}

// ====== Tab 2: 验证码记录（sys_email_verify_codes）======
const codeLoading = ref(false)
const codeData = ref<any[]>([])
const codePagination = reactive({
	current: 1, pageSize: 20, total: 0,
	showPageSize: true, pageSizeOptions: [10, 20, 50],
})
const codeFilter = reactive({ email: '', purpose: '', used: '' })

const purposeColor: Record<string, string> = { register: 'arcoblue', reset_password: 'orange', change_email: 'cyan' }
const purposeLabel: Record<string, string> = { register: '注册', reset_password: '重置密码', change_email: '更换邮箱' }

const codeColumns: TableColumnData[] = [
	{ title: 'ID', dataIndex: 'id', width: 80 },
	{ title: '邮箱', dataIndex: 'email', width: 220, ellipsis: true },
	{
		title: '验证码', dataIndex: 'code', width: 110,
		render({ record }) {
			return h('span', { style: 'font-family:monospace;letter-spacing:2px;font-weight:600;color:#165dff' }, record.code)
		},
	},
	{
		title: '用途', dataIndex: 'purpose', width: 100,
		render({ record }) {
			return h(Tag, { color: purposeColor[record.purpose] || 'gray', size: 'small' }, () => purposeLabel[record.purpose] || record.purpose)
		},
	},
	{ title: '有效期至', dataIndex: 'expires_at', width: 170 },
	{
		title: '是否已用', dataIndex: 'used_at', width: 100,
		render({ record }) {
			return h(Tag, { color: record.used_at ? 'green' : 'gray', size: 'small' }, () => record.used_at ? '已使用' : '未使用')
		},
	},
	{ title: '申请时间', dataIndex: 'created_at', width: 170 },
]

async function fetchCodeData() {
	codeLoading.value = true
	try {
		const params: Record<string, any> = { page: codePagination.current, page_size: codePagination.pageSize }
		if (codeFilter.email) params.email = codeFilter.email
		if (codeFilter.purpose) params.purpose = codeFilter.purpose
		if (codeFilter.used) params.used = codeFilter.used
		const res: any = await request.get('/admin/email/verify-codes', { params })
		const raw = res.data?.data
		codeData.value = (Array.isArray(raw?.list) ? raw.list : []).filter(Boolean)
		codePagination.total = Number(raw?.total) || 0
	} catch {
		codeData.value = []
		codePagination.total = 0
	} finally {
		codeLoading.value = false
	}
}

function handleCodeFilter() { codePagination.current = 1; fetchCodeData() }
function handleCodeReset() {
	codeFilter.email = ''
	codeFilter.purpose = ''
	codeFilter.used = ''
	codePagination.current = 1
	fetchCodeData()
}

// 切 Tab 懒加载
const loaded = reactive({ 'send-logs': false, 'verify-codes': false })
watch(activeTab, (v) => {
	if (v === 'verify-codes' && !loaded['verify-codes']) {
		loaded['verify-codes'] = true
		fetchCodeData()
	}
})

onMounted(() => {
	loaded['send-logs'] = true
	fetchSendData()
})
</script>

<template>
	<div class="page-table">
		<PageHeader title="邮件发送记录" description="查看邮件实际发送记录与验证码申请记录，排查邮件是否成功发出" />

		<a-tabs v-model:active-key="activeTab">
			<!-- 邮件发送记录 -->
			<a-tab-pane key="send-logs" title="邮件发送记录">
				<a-card :bordered="false" class="mb-4">
					<a-space wrap>
						<a-select v-model="sendFilter.status" placeholder="状态" allow-clear style="width: 140px" @change="handleSendFilter">
							<a-option value="sent">已发送</a-option>
							<a-option value="failed">发送失败</a-option>
						</a-select>
						<a-input v-model="sendFilter.recipient" placeholder="收件人" allow-clear style="width: 220px" @keydown.enter="handleSendFilter" />
						<a-range-picker v-model="sendFilter.date_range" style="width: 260px" format="YYYY-MM-DD" />
						<a-button type="primary" @click="handleSendFilter">搜索</a-button>
						<a-button @click="handleSendReset">重置</a-button>
					</a-space>
				</a-card>

				<a-card :bordered="false">
					<a-table
						:columns="sendColumns"
						:data="sendData"
						:loading="sendLoading"
						:scroll="{ x: 1300 }"
						:bordered="false"
						:stripe="true"
						size="small"
						:pagination="false"
						row-key="id"
					/>
					<div class="table-footer">
						<a-pagination
							v-model:current="sendPagination.current"
							v-model:page-size="sendPagination.pageSize"
							:total="sendPagination.total"
							:page-size-options="sendPagination.pageSizeOptions"
							show-page-size
							@change="fetchSendData"
							@page-size-change="(s: number) => { sendPagination.pageSize = s; sendPagination.current = 1; fetchSendData() }"
						/>
					</div>
				</a-card>
			</a-tab-pane>

			<!-- 验证码记录 -->
			<a-tab-pane key="verify-codes" title="验证码记录">
				<a-card :bordered="false" class="mb-4">
					<a-space wrap>
						<a-input v-model="codeFilter.email" placeholder="邮箱" allow-clear style="width: 220px" @keydown.enter="handleCodeFilter" />
						<a-select v-model="codeFilter.purpose" placeholder="用途" allow-clear style="width: 140px" @change="handleCodeFilter">
							<a-option value="register">注册</a-option>
							<a-option value="reset_password">重置密码</a-option>
							<a-option value="change_email">更换邮箱</a-option>
						</a-select>
						<a-select v-model="codeFilter.used" placeholder="是否已用" allow-clear style="width: 140px" @change="handleCodeFilter">
							<a-option value="true">已使用</a-option>
							<a-option value="false">未使用</a-option>
						</a-select>
						<a-button type="primary" @click="handleCodeFilter">搜索</a-button>
						<a-button @click="handleCodeReset">重置</a-button>
					</a-space>
				</a-card>

				<a-card :bordered="false">
					<a-table
						:columns="codeColumns"
						:data="codeData"
						:loading="codeLoading"
						:scroll="{ x: 1000 }"
						:bordered="false"
						:stripe="true"
						size="small"
						:pagination="false"
						row-key="id"
					/>
					<div class="table-footer">
						<a-pagination
							v-model:current="codePagination.current"
							v-model:page-size="codePagination.pageSize"
							:total="codePagination.total"
							:page-size-options="codePagination.pageSizeOptions"
							show-page-size
							@change="fetchCodeData"
							@page-size-change="(s: number) => { codePagination.pageSize = s; codePagination.current = 1; fetchCodeData() }"
						/>
					</div>
				</a-card>
			</a-tab-pane>
		</a-tabs>

		<!-- 详情抽屉 -->
		<a-drawer v-model:visible="showDetail" :width="560" title="邮件发送详情">
			<a-spin :loading="detailLoading" class="w-full">
				<template v-if="detailRecord">
					<a-descriptions :column="1" bordered size="medium">
						<a-descriptions-item label="收件人">{{ detailRecord.recipient }}</a-descriptions-item>
						<a-descriptions-item label="主题">{{ detailRecord.subject }}</a-descriptions-item>
						<a-descriptions-item label="模板">{{ detailRecord.template_code || '-' }}</a-descriptions-item>
						<a-descriptions-item label="状态">
							<Tag :color="statusColor[detailRecord.status] || 'gray'" size="small">{{ statusLabel[detailRecord.status] || detailRecord.status }}</Tag>
						</a-descriptions-item>
						<a-descriptions-item label="重试次数">{{ detailRecord.retry_count }}</a-descriptions-item>
						<a-descriptions-item label="发送时间">{{ detailRecord.sent_at || '-' }}</a-descriptions-item>
						<a-descriptions-item label="租户 / 用户">{{ detailRecord.tenant_id || '-' }} / {{ detailRecord.user_id || '-' }}</a-descriptions-item>
						<a-descriptions-item v-if="detailRecord.error_message" label="错误信息">
							<span style="color: #f53f3f">{{ detailRecord.error_message }}</span>
						</a-descriptions-item>
					</a-descriptions>
					<div v-if="detailRecord.body" class="mt-4">
						<h4 style="margin-bottom: 8px; color: var(--ta-text-primary)">邮件正文</h4>
						<pre class="email-body">{{ detailRecord.body }}</pre>
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
.email-body {
	background: var(--color-fill-2, #f7f8fa);
	padding: 12px;
	border-radius: 6px;
	font-size: 12px;
	max-height: 400px;
	overflow: auto;
	white-space: pre-wrap;
	word-break: break-all;
}
</style>
