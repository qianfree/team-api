<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const statusFilter = ref<string | undefined>(undefined)
const typeFilter = ref<string | undefined>(undefined)

const typeColorMap: Record<string, string> = { feature: 'arcoblue', fix: 'orangered', improvement: 'green', breaking: 'red' }
const typeLabelMap: Record<string, string> = { feature: '新功能', fix: '修复', improvement: '优化', breaking: '重大变更' }
const statusColorMap: Record<string, string> = { draft: undefined, published: 'green' }
const statusLabelMap: Record<string, string> = { draft: '草稿', published: '已发布' }

const typeOptions = [
  { label: '新功能', value: 'feature' },
  { label: '修复', value: 'fix' },
  { label: '优化', value: 'improvement' },
  { label: '重大变更', value: 'breaking' },
]
const statusOptions = [
  { label: '全部', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
]
const typeFilterOptions = [
  { label: '全部', value: '' },
  ...typeOptions,
]

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '版本', dataIndex: 'version', width: 100 },
  { title: '标题', dataIndex: 'title', width: 200, ellipsis: true, tooltip: true },
  { title: '类型', dataIndex: 'type', width: 100, render({ record }) {
    return h(Tag, { color: typeColorMap[record.type] || undefined, size: 'small' }, () => typeLabelMap[record.type] || record.type)
  }},
  { title: '状态', dataIndex: 'status', width: 90, render({ record }) {
    return h(Tag, { color: statusColorMap[record.status] || undefined, size: 'small' }, () => statusLabelMap[record.status] || record.status)
  }},
  { title: '发布时间', dataIndex: 'published_at', width: 160, render({ record }) { return record.published_at || '-' } },
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 200, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEditModal(record) }, () => '编辑'),
        record.status === 'draft' ? h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => publishChangelog(record) }, () => '发布') : null,
        h(Button, { size: 'small', type: 'text', status: 'danger', onClick: () => deleteChangelog(record) }, () => '删除'),
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (statusFilter.value) params.status = statusFilter.value
    if (typeFilter.value) params.type = typeFilter.value
    const res = await request.get('/admin/changelogs', { params })
    const d = res.data?.data
    data.value = d?.list || []
    pagination.total = d?.total || 0
  } catch {
    // error auto-displayed by interceptor
  } finally {
    loading.value = false
  }
}

const showModal = ref(false)
const editingId = ref<number | null>(null)
const formLoading = ref(false)
const form = reactive({
  version: '',
  title: '',
  content: '',
  type: 'feature' as string,
})

function resetForm() {
  Object.assign(form, { version: '', title: '', content: '', type: 'feature' })
}

function openCreateModal() {
  editingId.value = null
  resetForm()
  showModal.value = true
}

function openEditModal(row: any) {
  editingId.value = row.id
  Object.assign(form, {
    version: row.version || '',
    title: row.title || '',
    content: row.content || '',
    type: row.type || 'feature',
  })
  showModal.value = true
}

async function handleSubmit() {
  if (!form.version.trim()) { Message.warning('请输入版本号'); return }
  if (!form.title.trim()) { Message.warning('请输入标题'); return }
  if (!form.content.trim()) { Message.warning('请输入内容'); return }
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/changelogs/${editingId.value}`, form)
      Message.success('更新成功')
    } else {
      await request.post('/admin/changelogs', form)
      Message.success('创建成功')
    }
    showModal.value = false
    fetchList()
  } catch {
    // error auto-displayed
  } finally {
    formLoading.value = false
  }
}

function publishChangelog(row: any) {
  Modal.confirm({
    title: '确认发布',
    content: `确定要发布版本「${row.version}」的更新日志吗？发布后用户将可以看到。`,
    okText: '确认发布',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.post(`/admin/changelogs/${row.id}/publish`)
        Message.success('发布成功')
        fetchList()
      } catch { /* auto-displayed */ }
    },
  })
}

function deleteChangelog(row: any) {
  Modal.confirm({
    title: '确认删除',
    content: `确定要删除版本「${row.version}」的更新日志吗？此操作不可恢复。`,
    okText: '确认删除',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.delete(`/admin/changelogs/${row.id}`)
        Message.success('已删除')
        fetchList()
      } catch { /* auto-displayed */ }
    },
  })
}

onMounted(fetchList)
</script>

<template>
  <div>
    <PageHeader title="更新日志" description="管理系统版本更新日志">
      <template #actions>
        <AButton type="primary" @click="openCreateModal">创建日志</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>日志列表</span>
          <div class="flex gap-2">
            <ASelect v-model="typeFilter" :options="typeFilterOptions" style="width: 120px" allow-clear placeholder="类型筛选" @change="() => { pagination.current = 1; fetchList() }" />
            <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear placeholder="状态筛选" @change="() => { pagination.current = 1; fetchList() }" />
          </div>
        </div>
      </template>
      <ATable :columns="columns" :data="data" :loading="loading" row-key="id" :scroll="{ x: 1200 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <ADrawer v-model:visible="showModal" :title="editingId ? '编辑更新日志' : '创建更新日志'" :width="560" :mask-closable="false" :footer="true" unmount-on-close>
      <AForm :model="form" :auto-label-width="true" layout="vertical">
        <AFormItem label="版本号" required>
          <AInput v-model="form.version" placeholder="例如: v1.2.0" />
        </AFormItem>
        <AFormItem label="标题" required>
          <AInput v-model="form.title" placeholder="请输入更新标题" />
        </AFormItem>
        <AFormItem label="类型">
          <ASelect v-model="form.type" :options="typeOptions" />
        </AFormItem>
        <AFormItem label="内容" required>
          <ATextarea v-model="form.content" :auto-size="{ minRows: 6, maxRows: 20 }" placeholder="Markdown 格式的更新内容" />
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
