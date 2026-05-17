<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const announcements = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const statusFilter = ref<string | undefined>(undefined)
const statusOptions = [
  { label: '全部', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已归档', value: 'archived' },
]

const typeOptions = [
  { label: '通知', value: 'info' },
  { label: '警告', value: 'warning' },
  { label: '重要', value: 'important' },
]
const typeColorMap: Record<string, string> = { info: 'arcoblue', warning: 'orangered', important: 'red' }
const typeLabelMap: Record<string, string> = { info: '通知', warning: '警告', important: '重要' }

const positionOptions = [
  { label: '登录页', value: 'login' },
  { label: '控制台', value: 'console' },
  { label: '两者都显示', value: 'both' },
]
const positionLabelMap: Record<string, string> = { login: '登录页', console: '控制台', both: '两者' }

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '标题', dataIndex: 'title', width: 180, ellipsis: true, tooltip: true },
  { title: '类型', dataIndex: 'type', width: 80, render({ record }) {
    return h(Tag, { color: typeColorMap[record.type] || undefined, size: 'small' }, () => typeLabelMap[record.type] || record.type)
  }},
  { title: '状态', dataIndex: 'status', width: 80, render({ record }) {
    const colorMap: Record<string, string> = { draft: undefined, published: 'green', archived: 'orangered' }
    const labelMap: Record<string, string> = { draft: '草稿', published: '已发布', archived: '已归档' }
    return h(Tag, { color: colorMap[record.status] || undefined, size: 'small' }, () => labelMap[record.status] || record.status)
  }},
  { title: '置顶', dataIndex: 'is_pinned', width: 70, render({ record }) {
    return h(Tag, { color: record.is_pinned ? 'orangered' : undefined, size: 'small' }, () => record.is_pinned ? '置顶' : '-')
  }},
  { title: '展示位置', dataIndex: 'display_position', width: 100, render({ record }) {
    return h('span', {}, positionLabelMap[record.display_position] || record.display_position)
  }},
  { title: '生效时间', dataIndex: 'effective_at', width: 160, render({ record }) { return record.effective_at || '-' } },
  { title: '过期时间', dataIndex: 'expires_at', width: 160, render({ record }) { return record.expires_at || '-' } },
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 220, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEditModal(record) }, () => '编辑'),
        record.status === 'draft' ? h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => publishAnnouncement(record) }, () => '发布') : null,
        record.status === 'published' ? h(Button, { size: 'small', type: 'text', status: 'warning', onClick: () => archiveAnnouncement(record) }, () => '归档') : null,
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    const res = await request.get('/admin/announcements', { params })
    const data = res.data?.data
    announcements.value = data?.list || []
    pagination.total = data?.total || 0
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    loading.value = false
  }
}

// Create / Edit modal
const showModal = ref(false)
const editingId = ref<number | null>(null)
const formLoading = ref(false)
const form = reactive({
  title: '',
  type: 'info' as string,
  content: '',
  status: 'draft' as string,
  is_pinned: false,
  display_position: 'both' as string,
  effective_at: '',
  expires_at: '',
})

function resetForm() {
  Object.assign(form, {
    title: '', type: 'info', content: '', status: 'draft',
    is_pinned: false, display_position: 'both', effective_at: '', expires_at: '',
  })
}

function openCreateModal() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEditModal(row: any) {
  editingId.value = row.id
  Object.assign(form, {
    title: row.title || '',
    type: row.type || 'info',
    content: row.content || '',
    status: row.status || 'draft',
    is_pinned: !!row.is_pinned,
    display_position: row.display_position || 'both',
    effective_at: row.effective_at || '',
    expires_at: row.expires_at || '',
  })
  showModal.value = true
}

async function handleSubmit() {
  if (!form.title.trim()) {
    Message.warning('请输入标题')
    return
  }
  if (!form.content.trim()) {
    Message.warning('请输入内容')
    return
  }
  formLoading.value = true
  try {
    const payload = {
      ...form,
      effective_at: form.effective_at || null,
      expires_at: form.expires_at || null,
    }
    if (editingId.value) {
      await request.put(`/admin/announcements/${editingId.value}`, payload)
      Message.success('更新成功')
    } else {
      await request.post('/admin/announcements', payload)
      Message.success('创建成功')
    }
    showModal.value = false
    fetchList()
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    formLoading.value = false
  }
}

async function publishAnnouncement(row: any) {
  Modal.confirm({
    title: '确认发布',
    content: `确定要发布公告「${row.title}」吗？发布后将立即对用户可见。`,
    okText: '确认发布',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.put(`/admin/announcements/${row.id}/publish`)
        Message.success('发布成功')
        fetchList()
      } catch {
        // error auto-displayed by Axios interceptor
      }
    },
  })
}

async function archiveAnnouncement(row: any) {
  Modal.confirm({
    title: '确认归档',
    content: `确定要归档公告「${row.title}」吗？归档后用户将无法看到此公告。`,
    okText: '确认归档',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.put(`/admin/announcements/${row.id}/archive`)
        Message.success('已归档')
        fetchList()
      } catch {
        // error auto-displayed by Axios interceptor
      }
    },
  })
}

onMounted(fetchList)
</script>

<template>
  <div>
    <PageHeader title="公告管理" description="管理系统公告，控制发布范围和展示位置">
      <template #actions>
        <AButton type="primary" @click="openCreateModal">创建公告</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>公告列表</span>
          <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear placeholder="状态筛选" @change="() => { pagination.current = 1; fetchList() }" />
        </div>
      </template>
      <ATable :columns="columns" :data="announcements" :loading="loading" row-key="id" :scroll="{ x: 1400 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <!-- Create/Edit Drawer -->
    <ADrawer v-model:visible="showModal" :title="editingId ? '编辑公告' : '创建公告'" :width="560" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem label="标题" required>
          <AInput v-model="form.title" placeholder="请输入公告标题" />
        </AFormItem>
        <AFormItem label="类型">
          <ASelect v-model="form.type" :options="typeOptions" />
        </AFormItem>
        <AFormItem label="内容" required>
          <ATextarea v-model="form.content" :auto-size="{ minRows: 6, maxRows: 16 }" placeholder="支持 Markdown 格式，可直接粘贴 HTML 代码" />
        </AFormItem>
        <AFormItem v-if="!editingId" label="状态">
          <ASelect v-model="form.status" :options="[{ label: '草稿', value: 'draft' }, { label: '发布', value: 'published' }]" />
        </AFormItem>
        <AFormItem label="置顶">
          <ASwitch v-model="form.is_pinned" />
        </AFormItem>
        <AFormItem label="展示位置">
          <ASelect v-model="form.display_position" :options="positionOptions" />
        </AFormItem>
        <AFormItem label="生效时间">
          <ADatePicker v-model="form.effective_at" show-time format="YYYY-MM-DD HH:mm:ss" class="w-full" placeholder="留空则立即生效" />
        </AFormItem>
        <AFormItem label="过期时间">
          <ADatePicker v-model="form.expires_at" show-time format="YYYY-MM-DD HH:mm:ss" class="w-full" placeholder="留空则永不过期" />
        </AFormItem>
      </AForm>
      <template #footer>
        <Space>
          <AButton @click="showModal = false">取消</AButton>
          <AButton type="primary" :loading="formLoading" @click="handleSubmit">确认</AButton>
        </Space>
      </template>
    </ADrawer>
  </div>
</template>
