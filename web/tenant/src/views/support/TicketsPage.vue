<script setup lang="ts">
import { ref, reactive, onMounted, computed, h } from 'vue'
import type { DataTableColumns } from 'naive-ui'
import { NInput } from 'naive-ui'
import BaseModal from '@/components/common/BaseModal.vue'
import ResponsiveDataTable from '@/components/common/ResponsiveDataTable.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
import { renderBadge, tableScrollX } from '@/utils/renderUtils'
import request from '@/utils/request'
import { toast } from '@/utils/toast'
import { useExport } from '@/composables/useExport'

interface Ticket {
	id: number
	title: string
	category: string
	urgency: string
	status: string
	description: string
	assigned_admin: string
	created_at: string
	updated_at: string
}

interface Reply {
	id: number
	content: string
	is_admin: boolean
	author_name: string
	created_at: string
}

const tickets = ref<Ticket[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)

const showExportDropdown = ref(false)
const { exporting, exportFile } = useExport({
	url: '/tenant/tickets/export',
})

const showCreateModal = ref(false)
const createLoading = ref(false)
const createForm = reactive({
	category: 'technical' as string,
	title: '',
	description: '',
	urgency: 'normal' as string,
})

const showDetailModal = ref(false)
const detailLoading = ref(false)
const detailTicket = ref<Ticket | null>(null)
const detailReplies = ref<Reply[]>([])
const detailReplyContent = ref('')
const detailReplyLoading = ref(false)

const categoryOptions = [
	{ label: '技术支持', value: 'technical' },
	{ label: '账单问题', value: 'billing' },
	{ label: '功能建议', value: 'feature_request' },
	{ label: '其他', value: 'other' },
]

const urgencyOptions = [
	{ label: '低', value: 'low' },
	{ label: '普通', value: 'normal' },
	{ label: '高', value: 'high' },
	{ label: '紧急', value: 'urgent' },
]

const statusLabel: Record<string, string> = {
	pending: '待处理',
	processing: '处理中',
	replied: '已回复',
	closed: '已关闭',
	reopened: '已重新打开',
}

const statusBadgeClass: Record<string, string> = {
	pending: 'badge-primary',
	processing: 'badge-warning',
	replied: 'badge-success',
	closed: 'badge-gray',
	reopened: 'badge-purple',
}

const categoryLabel: Record<string, string> = {
	technical: '技术支持',
	billing: '账单问题',
	feature_request: '功能建议',
	other: '其他',
}

const urgencyLabel: Record<string, string> = {
	low: '低',
	normal: '普通',
	high: '高',
	urgent: '紧急',
}

const urgencyBadgeClass: Record<string, string> = {
	low: 'badge-gray',
	normal: 'badge-primary',
	high: 'badge-warning',
	urgent: 'badge-danger',
}


async function fetchTickets() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/tickets', {
			params: { page: page.value, page_size: pageSize.value },
		})
		const raw = res.data?.data
		tickets.value = Array.isArray(raw) ? raw : (raw?.data || raw?.list || [])
		total.value = raw?.total || 0
	} catch {
		tickets.value = []
		total.value = 0
	} finally {
		loading.value = false
	}
}

async function createTicket() {
	if (!createForm.title.trim()) { toast.warning('请输入标题'); return }
	if (!createForm.description.trim()) { toast.warning('请输入描述'); return }
	createLoading.value = true
	try {
		await request.post('/tenant/tickets', {
			category: createForm.category,
			title: createForm.title,
			description: createForm.description,
			urgency: createForm.urgency,
		})
		showCreateModal.value = false
		createForm.category = 'technical'
		createForm.title = ''
		createForm.description = ''
		createForm.urgency = 'normal'
		fetchTickets()
	} catch {
	} finally {
		createLoading.value = false
	}
}

async function openDetail(ticket: Ticket) {
	detailTicket.value = ticket
	detailReplies.value = []
	detailReplyContent.value = ''
	showDetailModal.value = true
	detailLoading.value = true
	try {
		const res: any = await request.get(`/tenant/tickets/${ticket.id}`)
		const data = res.data?.data
		detailTicket.value = data?.ticket || data || ticket
		detailReplies.value = data?.replies || []
	} catch {
	} finally {
		detailLoading.value = false
	}
}

async function handleDetailReply() {
	if (!detailTicket.value || !detailReplyContent.value.trim()) return
	detailReplyLoading.value = true
	try {
		await request.post(`/tenant/tickets/${detailTicket.value.id}/reply`, {
			content: detailReplyContent.value,
		})
		detailReplyContent.value = ''
		// Refresh detail
		const res: any = await request.get(`/tenant/tickets/${detailTicket.value!.id}`)
		const data = res.data?.data
		detailTicket.value = data?.ticket || data || detailTicket.value
		detailReplies.value = data?.replies || []
		fetchTickets()
	} catch {
	} finally {
		detailReplyLoading.value = false
	}
}

async function closeTicket() {
	if (!detailTicket.value) return
	try {
		await request.post(`/tenant/tickets/${detailTicket.value.id}/close`)
		detailTicket.value = { ...detailTicket.value, status: 'closed' }
		fetchTickets()
	} catch {
	}
}

async function reopenTicket() {
	if (!detailTicket.value) return
	try {
		await request.post(`/tenant/tickets/${detailTicket.value.id}/reopen`)
		detailTicket.value = { ...detailTicket.value, status: 'reopened' }
		fetchTickets()
	} catch {
	}
}

// NDataTable 列定义
const columns = computed<DataTableColumns<Ticket>>(() => [
	{ title: 'ID', key: 'id', width: 70, render: (row) => h('span', { class: 'font-mono text-xs text-gray-500' }, `#${row.id}`) },
	{ title: '分类', key: 'category', width: 120, render: (row) => h('span', { class: 'text-sm text-gray-600' }, categoryLabel[row.category] || row.category) },
	{
		title: '标题',
		key: 'title',
		width: 260,
		render: (row) => h('span', { class: 'font-medium text-gray-900 max-w-[240px] truncate block' }, row.title),
	},
	{ title: '优先级', key: 'urgency', width: 100, render: (row) => renderBadge(row.urgency, urgencyLabel, urgencyBadgeClass) },
	{ title: '状态', key: 'status', width: 110, render: (row) => renderBadge(row.status, statusLabel, statusBadgeClass) },
	{ title: '处理人', key: 'assigned_admin', width: 140, render: (row) => h('span', { class: 'text-sm text-gray-500' }, row.assigned_admin || '暂未分配') },
	{
		title: '创建时间',
		key: 'created_at',
		width: 180,
		render: (row) => h('span', { class: 'text-xs text-gray-500' }, row.created_at ? new Date(row.created_at).toLocaleString() : '-'),
	},
])

function handlePageSizeChange() {
	page.value = 1
	fetchTickets()
}

onMounted(() => {
	fetchTickets()
})
</script>

<template>
	<div class="viewport-table-page space-y-6">
		<!-- Page Header -->
		<div class="page-header flex items-center justify-between">
			<div>
				<h1 class="page-title">工单中心</h1>
				<p class="page-description">提交和管理您的支持工单</p>
			</div>
			<div class="flex items-center gap-2">
				<!-- Export dropdown -->
				<div class="relative inline-block">
					<button class="btn btn-secondary" :disabled="exporting" @click="showExportDropdown = !showExportDropdown">
						<svg v-if="!exporting" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3"/></svg>
						<svg v-else class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
						导出
					</button>
					<div v-if="showExportDropdown" class="absolute right-0 mt-2 w-36 bg-white rounded-xl border shadow-lg py-1 z-50">
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('csv'); showExportDropdown = false">导出 CSV</div>
						<div class="px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 cursor-pointer" @click="exportFile('xlsx'); showExportDropdown = false">导出 Excel</div>
					</div>
				</div>
				<button class="btn btn-primary" @click="showCreateModal = true">
					<Icon name="plus" size="sm" />
					新建工单
				</button>
			</div>
		</div>

		<!-- Table -->
		<div class="viewport-table-panel p-0 overflow-hidden">
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
				:data="tickets"
				:row-key="(row: Ticket) => row.id"
				card-title-key="title"
				card-badge-key="status"
				card-subtitle-key="created_at"
				:card-fields="['id', 'category', 'urgency', 'assigned_admin']"
				:row-click="openDetail"
				@update:page="fetchTickets"
				@update:page-size="handlePageSizeChange"
			>
				<template #empty>
					<div class="empty-state">
						<Icon name="document" size="xl" class="empty-state-icon text-gray-300" />
						<p class="empty-state-title">暂无工单</p>
						<p class="empty-state-description">遇到问题？创建一个工单获取帮助</p>
						<button class="btn btn-primary mt-4" @click="showCreateModal = true">
							<Icon name="plus" size="sm" />
							新建工单
						</button>
					</div>
				</template>
			</ResponsiveDataTable>
		</div>

		<!-- Create Ticket Modal -->
		<BaseModal
			:show="showCreateModal"
			title="新建工单"
			width="wide"
			@close="showCreateModal = false"
		>
			<div class="space-y-4">
				<div>
					<label class="input-label">分类</label>
					<BaseSelect v-model="createForm.category" :options="categoryOptions" />
				</div>
				<div>
					<label class="input-label">标题 <span class="text-red-500">*</span></label>
					<n-input v-model:value="createForm.title" type="text" placeholder="请简要描述您的问题" :maxlength="200" />
				</div>
				<div>
					<label class="input-label">优先级</label>
					<BaseSelect v-model="createForm.urgency" :options="urgencyOptions" />
				</div>
				<div>
					<label class="input-label">详细描述 <span class="text-red-500">*</span></label>
					<n-input
						v-model:value="createForm.description"
						type="textarea"
						:rows="5"
						placeholder="请详细描述您遇到的问题，包括相关的操作步骤和错误信息"
						:maxlength="2000"
					/>
					<p class="input-hint">{{ createForm.description.length }} / 2000</p>
				</div>
			</div>
			<template #footer>
				<div class="flex justify-end gap-3">
					<button class="btn btn-secondary" @click="showCreateModal = false">取消</button>
					<button
						class="btn btn-primary"
						:disabled="createLoading || !createForm.title.trim() || !createForm.description.trim()"
						@click="createTicket"
					>
						{{ createLoading ? '提交中...' : '提交工单' }}
					</button>
				</div>
			</template>
		</BaseModal>

		<!-- Detail Modal -->
		<BaseModal
			:show="showDetailModal"
			:title="'工单 #' + (detailTicket?.id || '')"
			width="extra-wide"
			@close="showDetailModal = false"
		>
			<div v-if="detailLoading" class="text-center py-8">
				<div class="spinner mx-auto mb-3"></div>
				<p class="text-sm text-gray-500">加载中...</p>
			</div>
			<div v-else-if="detailTicket" class="space-y-4">
				<!-- Ticket Info -->
				<div class="p-4 bg-gray-50 rounded-xl space-y-3">
					<div class="flex items-start justify-between">
						<h3 class="text-lg font-semibold text-gray-900">{{ detailTicket.title }}</h3>
						<span class="badge" :class="statusBadgeClass[detailTicket.status] || 'badge-gray'">
							{{ statusLabel[detailTicket.status] || detailTicket.status }}
						</span>
					</div>
					<div class="flex items-center gap-4 text-sm text-gray-600">
						<span>分类: {{ categoryLabel[detailTicket.category] || detailTicket.category }}</span>
						<span>优先级:
							<span class="badge" :class="urgencyBadgeClass[detailTicket.urgency] || 'badge-gray'">
								{{ urgencyLabel[detailTicket.urgency] || detailTicket.urgency }}
							</span>
						</span>
						<span>处理人: {{ detailTicket.assigned_admin || '暂未分配' }}</span>
					</div>
					<div class="text-xs text-gray-500">
						创建: {{ detailTicket.created_at ? new Date(detailTicket.created_at).toLocaleString() : '-' }}
						<span class="mx-2">|</span>
						更新: {{ detailTicket.updated_at ? new Date(detailTicket.updated_at).toLocaleString() : '-' }}
					</div>
					<p class="text-sm text-gray-700 whitespace-pre-wrap">{{ detailTicket.description }}</p>

					<!-- Action buttons -->
					<div class="flex items-center gap-2 pt-2 border-t border-gray-200">
						<button
							v-if="detailTicket.status !== 'closed'"
							class="btn btn-secondary btn-sm"
							@click="closeTicket"
						>
							关闭工单
						</button>
						<button
							v-if="detailTicket.status === 'closed'"
							class="btn btn-primary btn-sm"
							@click="reopenTicket"
						>
							重新打开
						</button>
					</div>
				</div>

				<!-- Replies (chat-like layout: tenant on left, admin on right) -->
				<div class="space-y-3 max-h-96 overflow-y-auto pr-1">
					<h4 class="text-sm font-medium text-gray-700">对话记录</h4>
					<div v-if="detailReplies.length === 0" class="text-sm text-gray-400 text-center py-6">
						暂无回复，等待客服处理
					</div>
					<div
						v-for="reply in detailReplies"
						:key="reply.id"
						class="flex gap-3"
						:class="reply.is_admin ? 'flex-row-reverse' : 'flex-row'"
					>
						<!-- Avatar -->
						<div
							class="h-8 w-8 rounded-full flex items-center justify-center text-white text-xs font-medium flex-shrink-0"
							:class="reply.is_admin ? 'bg-gradient-to-r from-primary-500 to-primary-600' : 'bg-gradient-to-r from-gray-400 to-gray-500'"
						>
							{{ reply.is_admin ? (reply.author_name || '客').charAt(0) : '我' }}
						</div>
						<!-- Bubble -->
						<div
							class="max-w-[75%] rounded-2xl px-4 py-2.5"
							:class="reply.is_admin
								? 'bg-primary-50 border border-primary-100 rounded-tl-md'
								: 'bg-white border border-gray-200 rounded-tr-md'"
						>
							<div class="flex items-center gap-2 mb-1">
								<span class="text-xs font-medium" :class="reply.is_admin ? 'text-primary-600' : 'text-gray-600'">
									{{ reply.is_admin ? (reply.author_name || '客服') : '我' }}
								</span>
								<span class="text-xs text-gray-400">{{ reply.created_at ? new Date(reply.created_at).toLocaleString() : '' }}</span>
							</div>
							<p class="text-sm text-gray-700 whitespace-pre-wrap leading-relaxed">{{ reply.content }}</p>
						</div>
					</div>
				</div>

				<!-- Reply Form -->
				<div v-if="detailTicket.status !== 'closed'" class="border-t border-gray-200 pt-4">
					<label class="input-label">回复</label>
					<n-input
						v-model:value="detailReplyContent"
						type="textarea"
						:rows="3"
						placeholder="输入您的回复... (Ctrl + Enter 发送)"
						@keyup.ctrl.enter="handleDetailReply"
					/>
					<div class="flex justify-end mt-2">
						<button
							class="btn btn-primary btn-sm"
							:disabled="detailReplyLoading || !detailReplyContent.trim()"
							@click="handleDetailReply"
						>
							{{ detailReplyLoading ? '发送中...' : '回复' }}
						</button>
					</div>
				</div>
			</div>
		</BaseModal>
	</div>
</template>
