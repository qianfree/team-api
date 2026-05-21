<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import BaseModal from '@/components/common/BaseModal.vue'
import Icon from '@/components/common/Icon.vue'
import BaseSelect from '../../components/common/BaseSelect.vue'
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
const pageSize = 20
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

const totalPages = computed(() => Math.ceil(total.value / pageSize))

async function fetchTickets() {
	loading.value = true
	try {
		const res: any = await request.get('/tenant/tickets', {
			params: { page: page.value, page_size: pageSize },
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

function handlePageChange(newPage: number) {
	page.value = newPage
	fetchTickets()
}

onMounted(() => {
	fetchTickets()
})
</script>

<template>
	<div class="space-y-6">
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

		<!-- Loading -->
		<div v-if="loading" class="card p-8 text-center">
			<div class="spinner mx-auto mb-3"></div>
			<p class="text-sm text-gray-500">加载中...</p>
		</div>

		<!-- Empty -->
		<div v-else-if="tickets.length === 0" class="empty-state card">
			<Icon name="document" size="xl" class="empty-state-icon text-gray-300" />
			<p class="empty-state-title">暂无工单</p>
			<p class="empty-state-description">遇到问题？创建一个工单获取帮助</p>
			<button class="btn btn-primary mt-4" @click="showCreateModal = true">
				<Icon name="plus" size="sm" />
				新建工单
			</button>
		</div>

		<!-- Table -->
		<div v-else class="card p-0 overflow-hidden">
			<div class="table-container">
				<table class="table">
					<thead>
						<tr>
							<th>ID</th>
							<th>分类</th>
							<th>标题</th>
							<th>优先级</th>
							<th>状态</th>
							<th>处理人</th>
							<th>创建时间</th>
						</tr>
					</thead>
					<tbody>
						<tr
							v-for="ticket in tickets"
							:key="ticket.id"
							class="cursor-pointer hover:bg-primary-50/50"
							@click="openDetail(ticket)"
						>
							<td>
								<span class="font-mono text-xs text-gray-500">#{{ ticket.id }}</span>
							</td>
							<td>
								<span class="text-sm text-gray-600">{{ categoryLabel[ticket.category] || ticket.category }}</span>
							</td>
							<td>
								<span class="font-medium text-gray-900 max-w-[240px] truncate block">{{ ticket.title }}</span>
							</td>
							<td>
								<span class="badge" :class="urgencyBadgeClass[ticket.urgency] || 'badge-gray'">
									{{ urgencyLabel[ticket.urgency] || ticket.urgency }}
								</span>
							</td>
							<td>
								<span class="badge" :class="statusBadgeClass[ticket.status] || 'badge-gray'">
									{{ statusLabel[ticket.status] || ticket.status }}
								</span>
							</td>
							<td>
								<span class="text-sm text-gray-500">{{ ticket.assigned_admin || '暂未分配' }}</span>
							</td>
							<td>
								<span class="text-xs text-gray-500">{{ ticket.created_at ? new Date(ticket.created_at).toLocaleString() : '-' }}</span>
							</td>
						</tr>
					</tbody>
				</table>
			</div>

			<!-- Pagination -->
			<div v-if="totalPages > 1" class="flex items-center justify-between px-6 py-3 border-t border-gray-100">
				<span class="text-xs text-gray-500">共 {{ total }} 条记录</span>
				<div class="flex items-center gap-2">
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page <= 1"
						@click="handlePageChange(page - 1)"
					>
						上一页
					</button>
					<span class="text-sm text-gray-600">{{ page }} / {{ totalPages }}</span>
					<button
						class="btn btn-ghost btn-sm"
						:disabled="page >= totalPages"
						@click="handlePageChange(page + 1)"
					>
						下一页
					</button>
				</div>
			</div>
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
					<input v-model="createForm.title" type="text" class="input" placeholder="请简要描述您的问题" maxlength="200" />
				</div>
				<div>
					<label class="input-label">优先级</label>
					<BaseSelect v-model="createForm.urgency" :options="urgencyOptions" />
				</div>
				<div>
					<label class="input-label">详细描述 <span class="text-red-500">*</span></label>
					<textarea
						v-model="createForm.description"
						class="input"
						rows="5"
						placeholder="请详细描述您遇到的问题，包括相关的操作步骤和错误信息"
						maxlength="2000"
					></textarea>
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
					<textarea
						v-model="detailReplyContent"
						class="input"
						rows="3"
						placeholder="输入您的回复... (Ctrl + Enter 发送)"
						@keyup.ctrl.enter="handleDetailReply"
					></textarea>
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
