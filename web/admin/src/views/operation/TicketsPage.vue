<script setup lang="ts">
import { ref, reactive, onMounted, h, computed } from 'vue'
import {
  Tag, Button, Space, Message, Modal, Textarea,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const tickets = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const statusFilter = ref<string | undefined>(undefined)
const categoryFilter = ref<string | undefined>(undefined)
const tenantFilter = ref('')

const statusOptions = [
  { label: '全部状态', value: '' },
  { label: '待处理', value: 'pending' },
  { label: '处理中', value: 'processing' },
  { label: '已回复', value: 'replied' },
  { label: '已关闭', value: 'closed' },
  { label: '已重开', value: 'reopened' },
]

const categoryOptions = [
  { label: '全部类型', value: '' },
  { label: '技术支持', value: 'technical' },
  { label: '账单问题', value: 'billing' },
  { label: '账号问题', value: 'account' },
  { label: '功能建议', value: 'feature' },
  { label: '其他', value: 'other' },
]

const statusColor: Record<string, string> = {
  pending: 'arcoblue',
  processing: 'orangered',
  replied: 'green',
  closed: undefined,
  reopened: 'purple',
}

const statusLabel: Record<string, string> = {
  pending: '待处理',
  processing: '处理中',
  replied: '已回复',
  closed: '已关闭',
  reopened: '已重开',
}

const categoryLabel: Record<string, string> = {
  technical: '技术支持',
  billing: '账单问题',
  account: '账号问题',
  feature: '功能建议',
  other: '其他',
}

const urgencyLabel: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  urgent: '紧急',
}

const urgencyColor: Record<string, string> = {
  low: undefined,
  medium: 'arcoblue',
  high: 'orangered',
  urgent: 'red',
}

const changeStatusOptions = [
  { label: '待处理', value: 'pending' },
  { label: '处理中', value: 'processing' },
  { label: '已回复', value: 'replied' },
  { label: '已关闭', value: 'closed' },
  { label: '已重开', value: 'reopened' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 70 },
  { title: '租户名称', dataIndex: 'tenant_name', width: 120, ellipsis: true },
  { title: '用户名称', dataIndex: 'user_name', width: 100, ellipsis: true },
  {
    title: '类型', dataIndex: 'category', width: 100,
    render({ record }) { return h(Tag, { size: 'small' }, () => categoryLabel[record.category] || record.category) },
  },
  { title: '标题', dataIndex: 'title', width: 180, ellipsis: true },
  {
    title: '紧急度', dataIndex: 'urgency', width: 80,
    render({ record }) { return h(Tag, { color: urgencyColor[record.urgency], size: 'small' }, () => urgencyLabel[record.urgency] || record.urgency) },
  },
  {
    title: '状态', dataIndex: 'status', width: 90,
    render({ record }) { return h(Tag, { color: statusColor[record.status], size: 'small' }, () => statusLabel[record.status] || record.status) },
  },
  { title: '处理人', dataIndex: 'assigned_admin_name', width: 100, ellipsis: true, render({ record }) { return record.assigned_admin_name || '-' } },
  {
    title: '创建时间', dataIndex: 'created_at', width: 170,
    render({ record }) { return record.created_at ? new Date(record.created_at).toLocaleString() : '-' },
  },
  {
    title: '操作', dataIndex: 'actions', width: 240, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 4 }, () => [
        h(Button, { size: 'small', onClick: () => openDetail(record) }, () => '查看'),
        h(Button, { size: 'small', onClick: () => openAssignModal(record) }, () => '分配'),
        h(Button, { size: 'small', onClick: () => openReplyModal(record) }, () => '回复'),
      ])
    },
  },
]

async function fetchTickets() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (categoryFilter.value) params.category = categoryFilter.value
    if (tenantFilter.value) params.tenant_id = tenantFilter.value
    const res = await request.get('/admin/tickets', { params })
    const data = res.data?.data
    tickets.value = data?.list || data?.data || []
    pagination.total = data?.total || 0
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    loading.value = false
  }
}

// === Detail Modal ===
const showDetailModal = ref(false)
const detailLoading = ref(false)
const detailTicket = ref<any>(null)
const detailReplies = ref<any[]>([])

async function openDetail(row: any) {
  detailTicket.value = row
  detailReplies.value = []
  showDetailModal.value = true
  detailLoading.value = true
  try {
    const res = await request.get(`/admin/tickets/${row.id}`)
    const data = res.data?.data
    detailTicket.value = data?.ticket || data || row
    detailReplies.value = data?.replies || []
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    detailLoading.value = false
  }
}

// === Assign Modal ===
const showAssignModal = ref(false)
const assignTicketId = ref<number | null>(null)
const assignForm = reactive({ admin_id: null as number | null })
const assignLoading = ref(false)
const adminList = ref<any[]>([])

async function openAssignModal(row: any) {
  assignTicketId.value = row.id
  assignForm.admin_id = row.assigned_admin_id || null
  showAssignModal.value = true
  if (adminList.value.length === 0) {
    try {
      const res = await request.get('/admin/users', { params: { page: 1, page_size: 100 } })
      const data = res.data?.data
      adminList.value = data?.list || data?.data || []
    } catch {
      adminList.value = []
    }
  }
}

async function handleAssign(done: () => void) {
  if (!assignForm.admin_id) { Message.warning('请选择管理员'); return }
  assignLoading.value = true
  try {
    await request.put(`/admin/tickets/${assignTicketId.value}/assign`, { admin_id: assignForm.admin_id })
    Message.success('分配成功')
    done()
    fetchTickets()
  } catch {
    return false
  } finally {
    assignLoading.value = false
  }
}

// === Reply Modal ===
const showReplyModal = ref(false)
const replyTicketId = ref<number | null>(null)
const replyForm = reactive({ content: '' })
const replyLoading = ref(false)

function openReplyModal(row: any) {
  replyTicketId.value = row.id
  replyForm.content = ''
  showReplyModal.value = true
}

async function handleReply(done: () => void) {
  if (!replyForm.content.trim()) { Message.warning('请输入回复内容'); return }
  replyLoading.value = true
  try {
    await request.post(`/admin/tickets/${replyTicketId.value}/reply`, { content: replyForm.content })
    Message.success('回复成功')
    done()
    fetchTickets()
  } catch {
    return false
  } finally {
    replyLoading.value = false
  }
}

// === Change Status ===
async function changeStatus(ticketId: number, status: string) {
  try {
    await request.put(`/admin/tickets/${ticketId}/status`, { status })
    Message.success('状态已更新')
    fetchTickets()
  } catch {
    // error auto-displayed by Axios interceptor
  }
}

// === Detail Reply (from detail modal) ===
const detailReplyContent = ref('')
const detailReplyLoading = ref(false)

async function handleDetailReply() {
  if (!detailTicket.value || !detailReplyContent.value.trim()) { Message.warning('请输入回复内容'); return }
  detailReplyLoading.value = true
  try {
    await request.post(`/admin/tickets/${detailTicket.value.id}/reply`, { content: detailReplyContent.value })
    Message.success('回复成功')
    detailReplyContent.value = ''
    // Refresh detail
    const res = await request.get(`/admin/tickets/${detailTicket.value.id}`)
    const data = res.data?.data
    detailTicket.value = data?.ticket || data || detailTicket.value
    detailReplies.value = data?.replies || []
    fetchTickets()
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    detailReplyLoading.value = false
  }
}

async function handleDetailChangeStatus(status: string) {
  if (!detailTicket.value) return
  try {
    await request.put(`/admin/tickets/${detailTicket.value.id}/status`, { status })
    Message.success('状态已更新')
    detailTicket.value = { ...detailTicket.value, status }
    fetchTickets()
  } catch {
    // error auto-displayed by Axios interceptor
  }
}

const adminOptions = computed(() =>
  adminList.value.map((a: any) => ({ label: a.username || a.email, value: a.id }))
)

onMounted(fetchTickets)
</script>

<template>
  <div>
    <PageHeader title="工单管理" description="查看和处理所有租户工单" />

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>工单列表</span>
          <ASpace>
            <AInput v-model="tenantFilter" placeholder="租户ID" allow-clear style="width: 120px" @clear="() => { pagination.current = 1; fetchTickets() }" />
            <ASelect v-model="categoryFilter" :options="categoryOptions" style="width: 130px" allow-clear @change="() => { pagination.current = 1; fetchTickets() }" />
            <ASelect v-model="statusFilter" :options="statusOptions" style="width: 130px" allow-clear @change="() => { pagination.current = 1; fetchTickets() }" />
          </ASpace>
        </div>
      </template>
      <ATable :columns="columns" :data="tickets" :loading="loading" row-key="id" :scroll="{ x: 1300 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchTickets" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchTickets() }" />
      </div>
    </ACard>

    <!-- Detail Modal -->
    <AModal v-model:visible="showDetailModal" title="工单详情" :width="720" :footer="false" :mask-closable="true">
      <div v-if="detailLoading" class="text-center py-8">加载中...</div>
      <div v-else-if="detailTicket" class="space-y-4">
        <!-- Ticket Info -->
        <div class="p-4 bg-gray-50 rounded-lg space-y-2">
          <div class="flex items-center justify-between">
            <h3 class="text-base font-semibold">{{ detailTicket.title }}</h3>
            <ASpace>
              <ATag :color="statusColor[detailTicket.status]">{{ statusLabel[detailTicket.status] || detailTicket.status }}</ATag>
              <ADropdown trigger="hover">
                <AButton size="mini">修改状态</AButton>
                <template #content>
                  <ADoption v-for="opt in changeStatusOptions" :key="opt.value" @click="handleDetailChangeStatus(opt.value)">{{ opt.label }}</ADoption>
                </template>
              </ADropdown>
            </ASpace>
          </div>
          <div class="grid grid-cols-3 gap-2 text-sm text-gray-600">
            <div>工单ID: {{ detailTicket.id }}</div>
            <div>类型: {{ categoryLabel[detailTicket.category] || detailTicket.category }}</div>
            <div>紧急度: <ATag :color="urgencyColor[detailTicket.urgency]" size="small">{{ urgencyLabel[detailTicket.urgency] || detailTicket.urgency }}</ATag></div>
            <div>租户: {{ detailTicket.tenant_name || '-' }}</div>
            <div>用户: {{ detailTicket.user_name || '-' }}</div>
            <div>处理人: {{ detailTicket.assigned_admin_name || '未分配' }}</div>
          </div>
          <div class="text-sm text-gray-700 mt-2 whitespace-pre-wrap">{{ detailTicket.description }}</div>
        </div>

        <!-- Replies -->
        <div class="space-y-3">
          <h4 class="text-sm font-medium text-gray-700">回复记录 ({{ detailReplies.length }})</h4>
          <div v-if="detailReplies.length === 0" class="text-sm text-gray-400 text-center py-4">暂无回复</div>
          <div v-for="reply in detailReplies" :key="reply.id" class="p-3 border rounded-lg" :class="reply.is_admin ? 'bg-blue-50 border-blue-200' : 'bg-gray-50 border-gray-200'">
            <div class="flex items-center justify-between mb-1">
              <span class="text-xs font-medium" :class="reply.is_admin ? 'text-blue-700' : 'text-gray-700'">
                {{ reply.author_name || (reply.is_admin ? '管理员' : '用户') }}
              </span>
              <span class="text-xs text-gray-400">{{ reply.created_at ? new Date(reply.created_at).toLocaleString() : '' }}</span>
            </div>
            <p class="text-sm text-gray-700 whitespace-pre-wrap">{{ reply.content }}</p>
          </div>
        </div>

        <!-- Reply Form -->
        <div class="border-t pt-4">
          <ATextarea v-model="detailReplyContent" placeholder="输入回复内容..." :auto-size="{ minRows: 3, maxRows: 6 }" />
          <div class="flex justify-end mt-2">
            <AButton type="primary" :loading="detailReplyLoading" :disabled="!detailReplyContent.trim()" @click="handleDetailReply">发送回复</AButton>
          </div>
        </div>
      </div>
    </AModal>

    <!-- Assign Modal -->
    <AModal v-model:visible="showAssignModal" title="分配工单" :width="450" :mask-closable="false" :on-before-ok="handleAssign" :ok-loading="assignLoading">
      <AForm :model="assignForm" layout="vertical">
        <AFormItem label="选择管理员" required>
          <ASelect v-model="assignForm.admin_id" :options="adminOptions" placeholder="请选择管理员" allow-search />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Reply Modal -->
    <AModal v-model:visible="showReplyModal" title="回复工单" :width="500" :mask-closable="false" :on-before-ok="handleReply" :ok-loading="replyLoading">
      <AForm :model="replyForm" layout="vertical">
        <AFormItem label="回复内容" required>
          <ATextarea v-model="replyForm.content" placeholder="请输入回复内容" :auto-size="{ minRows: 4, maxRows: 8 }" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>
