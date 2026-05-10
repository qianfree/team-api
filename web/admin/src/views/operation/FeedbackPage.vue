<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

// ============================================================
// Stats
// ============================================================
const stats = reactive({
  total: 0, pending: 0, acknowledged: 0, in_progress: 0, resolved: 0, closed: 0,
})

async function fetchStats() {
  try {
    const res = await request.get('/admin/feedbacks/stats')
    const d = res.data?.data
    if (d) {
      stats.total = d.total || 0
      stats.pending = d.pending || 0
      stats.acknowledged = d.acknowledged || 0
      stats.in_progress = d.in_progress || 0
      stats.resolved = d.resolved || 0
      stats.closed = d.closed || 0
    }
  } catch { /* auto-displayed */ }
}

const statCards = [
  { key: 'total', label: '总计', color: '' },
  { key: 'pending', label: '待处理', color: 'arcoblue' },
  { key: 'in_progress', label: '处理中', color: 'orangered' },
  { key: 'resolved', label: '已解决', color: 'green' },
  { key: 'closed', label: '已关闭', color: '' },
]

// ============================================================
// List
// ============================================================
const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const filters = reactive({ status: '' as string, category: '' as string, priority: '' as string })

const categoryLabelMap: Record<string, string> = { bug_report: 'Bug报告', feature_request: '功能请求', suggestion: '建议', complaint: '投诉' }
const categoryColorMap: Record<string, string> = { bug_report: 'orangered', feature_request: 'arcoblue', suggestion: 'green', complaint: 'red' }
const statusLabelMap: Record<string, string> = { pending: '待处理', acknowledged: '已确认', in_progress: '处理中', resolved: '已解决', closed: '已关闭' }
const statusColorMap: Record<string, string> = { pending: 'arcoblue', acknowledged: 'cyan', in_progress: 'orangered', resolved: 'green', closed: undefined }
const priorityLabelMap: Record<string, string> = { low: '低', normal: '普通', high: '高', critical: '紧急' }
const priorityColorMap: Record<string, string> = { low: undefined, normal: 'arcoblue', high: 'orangered', critical: 'red' }

const statusOptions = [
  { label: '全部', value: '' },
  { label: '待处理', value: 'pending' },
  { label: '已确认', value: 'acknowledged' },
  { label: '处理中', value: 'in_progress' },
  { label: '已解决', value: 'resolved' },
  { label: '已关闭', value: 'closed' },
]
const categoryOptions = [
  { label: '全部', value: '' },
  { label: 'Bug报告', value: 'bug_report' },
  { label: '功能请求', value: 'feature_request' },
  { label: '建议', value: 'suggestion' },
  { label: '投诉', value: 'complaint' },
]
const priorityOptions = [
  { label: '全部', value: '' },
  { label: '低', value: 'low' },
  { label: '普通', value: 'normal' },
  { label: '高', value: 'high' },
  { label: '紧急', value: 'critical' },
]
const replyStatusOptions = [
  { label: '已确认', value: 'acknowledged' },
  { label: '处理中', value: 'in_progress' },
  { label: '已解决', value: 'resolved' },
  { label: '已关闭', value: 'closed' },
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '租户', dataIndex: 'tenant_name', width: 120, ellipsis: true, tooltip: true },
  { title: '用户', dataIndex: 'user_display_name', width: 100, ellipsis: true, tooltip: true },
  { title: '类别', dataIndex: 'category', width: 90, render({ record }) {
    return h(Tag, { color: categoryColorMap[record.category] || undefined, size: 'small' }, () => categoryLabelMap[record.category] || record.category)
  }},
  { title: '优先级', dataIndex: 'priority', width: 80, render({ record }) {
    return h(Tag, { color: priorityColorMap[record.priority] || undefined, size: 'small' }, () => priorityLabelMap[record.priority] || record.priority)
  }},
  { title: '标题', dataIndex: 'title', width: 200, ellipsis: true, tooltip: true },
  { title: '状态', dataIndex: 'status', width: 90, render({ record }) {
    return h(Tag, { color: statusColorMap[record.status] || undefined, size: 'small' }, () => statusLabelMap[record.status] || record.status)
  }},
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 200, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openDetail(record) }, () => '查看'),
        record.status !== 'closed' && record.status !== 'resolved'
          ? h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => openReplyModal(record) }, () => '回复')
          : null,
        h(Button, { size: 'small', type: 'text', onClick: () => openStatusModal(record) }, () => '状态'),
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (filters.status) params.status = filters.status
    if (filters.category) params.category = filters.category
    if (filters.priority) params.priority = filters.priority
    const res = await request.get('/admin/feedbacks', { params })
    const d = res.data?.data
    data.value = d?.list || []
    pagination.total = d?.total || 0
  } catch { /* auto-displayed */ } finally {
    loading.value = false
  }
}

function onFilterChange() {
  pagination.current = 1
  fetchList()
}

// ============================================================
// Detail Modal
// ============================================================
const showDetailModal = ref(false)
const detailRecord = ref<any>(null)

function openDetail(row: any) {
  detailRecord.value = row
  showDetailModal.value = true
}

// ============================================================
// Reply Drawer
// ============================================================
const showReplyModal = ref(false)
const replyLoading = ref(false)
const replyForm = reactive({ reply: '', status: 'acknowledged' as string, resolution: '' })
const replyingId = ref<number | null>(null)

function openReplyModal(row: any) {
  replyingId.value = row.id
  Object.assign(replyForm, { reply: '', status: 'acknowledged', resolution: '' })
  showReplyModal.value = true
}

async function handleReply() {
  if (!replyForm.reply.trim()) { Message.warning('请输入回复内容'); return }
  replyLoading.value = true
  try {
    await request.post(`/admin/feedbacks/${replyingId.value}/reply`, replyForm)
    Message.success('回复成功')
    showReplyModal.value = false
    fetchList()
    fetchStats()
  } catch { /* auto-displayed */ } finally {
    replyLoading.value = false
  }
}

// ============================================================
// Update Status Modal
// ============================================================
const showStatusModal = ref(false)
const statusLoading = ref(false)
const statusForm = reactive({ status: '' as string, priority: '' as string })
const statusId = ref<number | null>(null)

function openStatusModal(row: any) {
  statusId.value = row.id
  Object.assign(statusForm, { status: row.status, priority: row.priority })
  showStatusModal.value = true
}

async function handleStatusUpdate() {
  if (!statusForm.status) { Message.warning('请选择状态'); return }
  statusLoading.value = true
  try {
    await request.put(`/admin/feedbacks/${statusId.value}/status`, statusForm)
    Message.success('状态已更新')
    showStatusModal.value = false
    fetchList()
    fetchStats()
  } catch { /* auto-displayed */ } finally {
    statusLoading.value = false
  }
}

onMounted(() => { fetchStats(); fetchList() })
</script>

<template>
  <div>
    <PageHeader title="反馈管理" description="查看和管理租户用户提交的反馈" />

    <!-- Stats Cards -->
    <div class="grid grid-cols-5 gap-4 mb-4">
      <ACard v-for="card in statCards" :key="card.key" :bordered="false" class="text-center">
        <div class="text-3xl font-bold" :class="card.color ? `text-${card.color}-500` : 'text-gray-700'">
          {{ (stats as any)[card.key] || 0 }}
        </div>
        <div class="text-sm text-gray-500 mt-1">{{ card.label }}</div>
      </ACard>
    </div>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>反馈列表</span>
          <div class="flex gap-2">
            <ASelect v-model="filters.status" :options="statusOptions" style="width: 110px" allow-clear placeholder="状态" @change="onFilterChange" />
            <ASelect v-model="filters.category" :options="categoryOptions" style="width: 110px" allow-clear placeholder="类别" @change="onFilterChange" />
            <ASelect v-model="filters.priority" :options="priorityOptions" style="width: 100px" allow-clear placeholder="优先级" @change="onFilterChange" />
          </div>
        </div>
      </template>
      <ATable :columns="columns" :data="data" :loading="loading" row-key="id" :scroll="{ x: 1300 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <!-- Detail Modal -->
    <AModal v-model:visible="showDetailModal" title="反馈详情" :width="680" :footer="false">
      <template v-if="detailRecord">
        <ADescriptions :column="2" bordered>
          <ADescriptionsItem label="ID">{{ detailRecord.id }}</ADescriptionsItem>
          <ADescriptionsItem label="租户">{{ detailRecord.tenant_name }}</ADescriptionsItem>
          <ADescriptionsItem label="用户">{{ detailRecord.user_display_name }}</ADescriptionsItem>
          <ADescriptionsItem label="类别">{{ categoryLabelMap[detailRecord.category] || detailRecord.category }}</ADescriptionsItem>
          <ADescriptionsItem label="优先级">{{ priorityLabelMap[detailRecord.priority] || detailRecord.priority }}</ADescriptionsItem>
          <ADescriptionsItem label="状态">{{ statusLabelMap[detailRecord.status] || detailRecord.status }}</ADescriptionsItem>
          <ADescriptionsItem label="标题" :span="2">{{ detailRecord.title }}</ADescriptionsItem>
          <ADescriptionsItem label="描述" :span="2">
            <div class="whitespace-pre-wrap">{{ detailRecord.description }}</div>
          </ADescriptionsItem>
          <ADescriptionsItem v-if="detailRecord.admin_reply" label="管理员回复" :span="2">
            <div class="whitespace-pre-wrap">{{ detailRecord.admin_reply }}</div>
          </ADescriptionsItem>
          <ADescriptionsItem v-if="detailRecord.resolution" label="解决方案" :span="2">
            {{ detailRecord.resolution }}
          </ADescriptionsItem>
          <ADescriptionsItem label="创建时间">{{ detailRecord.created_at }}</ADescriptionsItem>
          <ADescriptionsItem label="更新时间">{{ detailRecord.updated_at }}</ADescriptionsItem>
        </ADescriptions>
      </template>
    </AModal>

    <!-- Reply Drawer -->
    <ADrawer v-model:visible="showReplyModal" title="回复反馈" :width="520" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm :model="replyForm" layout="vertical">
        <AFormItem label="回复内容" required>
          <ATextarea v-model="replyForm.reply" :auto-size="{ minRows: 4, maxRows: 10 }" placeholder="请输入回复内容" />
        </AFormItem>
        <AFormItem label="更新状态">
          <ASelect v-model="replyForm.status" :options="replyStatusOptions" />
        </AFormItem>
        <AFormItem label="解决方案">
          <ATextarea v-model="replyForm.resolution" :auto-size="{ minRows: 2, maxRows: 6 }" placeholder="可选，解决方案摘要" />
        </AFormItem>
      </AForm>
      <template #footer>
        <Space>
          <AButton @click="showReplyModal = false">取消</AButton>
          <AButton type="primary" :loading="replyLoading" @click="handleReply">提交回复</AButton>
        </Space>
      </template>
    </ADrawer>

    <!-- Update Status Modal -->
    <AModal v-model:visible="showStatusModal" title="更新状态" @ok="handleStatusUpdate" :ok-loading="statusLoading">
      <AForm :model="statusForm" layout="vertical">
        <AFormItem label="状态" required>
          <ASelect v-model="statusForm.status" :options="statusOptions.filter(o => o.value)" />
        </AFormItem>
        <AFormItem label="优先级">
          <ASelect v-model="statusForm.priority" :options="priorityOptions.filter(o => o.value)" allow-clear />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>
