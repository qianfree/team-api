<script setup lang="ts">
import { ref, reactive, onMounted, h, computed } from 'vue'
import {
  Tag, Button, Space, Message, Modal,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import { marked } from 'marked'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const data = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const codeFilter = ref<string | undefined>(undefined)
const statusFilter = ref<string | undefined>(undefined)

const codeOptions = [
  { label: '全部', value: '' },
  { label: '用户协议', value: 'terms' },
  { label: '隐私政策', value: 'privacy' },
]
const statusOptions = [
  { label: '全部', value: '' },
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已归档', value: 'archived' },
]

const codeLabelMap: Record<string, string> = { terms: '用户协议', privacy: '隐私政策' }
const statusColorMap: Record<string, string | undefined> = { draft: undefined, published: 'green', archived: 'gray' }
const statusLabelMap: Record<string, string> = { draft: '草稿', published: '已发布', archived: '已归档' }

const columns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '协议类型', dataIndex: 'code', width: 110, render({ record }) {
    return codeLabelMap[record.code] || record.code
  }},
  { title: '版本', dataIndex: 'version', width: 90 },
  { title: '标题', dataIndex: 'title', width: 200, ellipsis: true, tooltip: true },
  { title: '状态', dataIndex: 'status', width: 90, render({ record }) {
    const parts = [
      h(Tag, { color: statusColorMap[record.status], size: 'small' }, () => statusLabelMap[record.status] || record.status),
    ]
    if (record.is_current) {
      parts.push(h(Tag, { color: 'arcoblue', size: 'small', style: 'margin-left:4px' }, () => '当前'))
    }
    return h(Space, { size: 4 }, () => parts)
  }},
  { title: '强制接受', dataIndex: 'force_accept', width: 90, render({ record }) {
    return record.force_accept ? '是' : '否'
  }},
  { title: '发布时间', dataIndex: 'published_at', width: 160, render({ record }) { return record.published_at || '-' } },
  { title: '创建时间', dataIndex: 'created_at', width: 160 },
  {
    title: '操作', dataIndex: 'actions', width: 260, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEditModal(record) }, () => record.status === 'draft' ? '编辑' : '查看'),
        record.status === 'draft' ? h(Button, { size: 'small', type: 'text', status: 'success', onClick: () => publishAgreement(record) }, () => '发布') : null,
        h(Button, { size: 'small', type: 'text', onClick: () => openAcceptances(record) }, () => '接受记录'),
        record.status === 'draft' ? h(Button, { size: 'small', type: 'text', status: 'danger', onClick: () => deleteAgreement(record) }, () => '删除') : null,
      ])
    },
  },
]

async function fetchList() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (codeFilter.value) params.code = codeFilter.value
    if (statusFilter.value) params.status = statusFilter.value
    const res = await request.get('/admin/agreements', { params })
    const d = res.data?.data
    data.value = d?.list || []
    pagination.total = d?.total || 0
  } catch {
    // error auto-displayed by interceptor
  } finally {
    loading.value = false
  }
}

// === Create / Edit Drawer ===
const showModal = ref(false)
const editingId = ref<number | null>(null)
const editingStatus = ref<string>('draft')
const formLoading = ref(false)
const form = reactive({
  code: 'terms' as string,
  version: '',
  title: '',
  content: '',
  summary: '',
  force_accept: true,
})

const isReadonly = computed(() => editingId.value !== null && editingStatus.value !== 'draft')

function resetForm() {
  Object.assign(form, { code: 'terms', version: '', title: '', content: '', summary: '', force_accept: true })
}

function openCreateModal() {
  editingId.value = null
  editingStatus.value = 'draft'
  resetForm()
  showModal.value = true
}

async function openEditModal(row: any) {
  editingId.value = row.id
  editingStatus.value = row.status
  // Fetch full detail (with content)
  try {
    const res = await request.get(`/admin/agreements/${row.id}`)
    const d = res.data?.data
    Object.assign(form, {
      code: d.code || 'terms',
      version: d.version || '',
      title: d.title || '',
      content: d.content || '',
      summary: d.summary || '',
      force_accept: d.force_accept ?? true,
    })
  } catch {
    Object.assign(form, {
      code: row.code || 'terms',
      version: row.version || '',
      title: row.title || '',
      content: '',
      summary: row.summary || '',
      force_accept: row.force_accept ?? true,
    })
  }
  showModal.value = true
}

async function handleSubmit() {
  if (!form.version.trim()) { Message.warning('请输入版本号'); return }
  if (!form.title.trim()) { Message.warning('请输入标题'); return }
  if (!form.content.trim()) { Message.warning('请输入内容'); return }
  formLoading.value = true
  try {
    if (editingId.value) {
      await request.put(`/admin/agreements/${editingId.value}`, {
        title: form.title,
        content: form.content,
        summary: form.summary,
        force_accept: form.force_accept,
      })
      Message.success('更新成功')
    } else {
      await request.post('/admin/agreements', form)
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

function publishAgreement(row: any) {
  Modal.confirm({
    title: '确认发布',
    content: `确定要发布「${codeLabelMap[row.code] || row.code}」版本 ${row.version} 吗？发布后将替换当前生效版本，所有用户需重新接受。`,
    okText: '确认发布',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.post(`/admin/agreements/${row.id}/publish`)
        Message.success('发布成功')
        fetchList()
      } catch { /* auto-displayed */ }
    },
  })
}

function deleteAgreement(row: any) {
  Modal.confirm({
    title: '确认删除',
    content: `确定要删除「${codeLabelMap[row.code] || row.code}」版本 ${row.version} 吗？此操作不可恢复。`,
    okText: '确认删除',
    cancelText: '取消',
    onOk: async () => {
      try {
        await request.delete(`/admin/agreements/${row.id}`)
        Message.success('已删除')
        fetchList()
      } catch { /* auto-displayed */ }
    },
  })
}

// === Acceptances Drawer ===
const showAcceptances = ref(false)
const acceptancesTitle = ref('')
const acceptancesLoading = ref(false)
const acceptancesData = ref<any[]>([])
const acceptancesPagination = reactive({ current: 1, pageSize: 20, total: 0 })
const currentAcceptanceId = ref<number | null>(null)

const acceptanceColumns: TableColumnData[] = [
  { title: 'ID', dataIndex: 'id', width: 60 },
  { title: '用户类型', dataIndex: 'user_type', width: 100, render({ record }) {
    return record.user_type === 'admin' ? '管理员' : '租户用户'
  }},
  { title: '用户ID', dataIndex: 'user_id', width: 90 },
  { title: 'IP 地址', dataIndex: 'ip_address', width: 140 },
  { title: '接受时间', dataIndex: 'created_at', width: 180 },
]

function openAcceptances(row: any) {
  currentAcceptanceId.value = row.id
  acceptancesTitle.value = `${codeLabelMap[row.code] || row.code} ${row.version} - 接受记录`
  acceptancesPagination.current = 1
  showAcceptances.value = true
  fetchAcceptances()
}

async function fetchAcceptances() {
  if (!currentAcceptanceId.value) return
  acceptancesLoading.value = true
  try {
    const params = { page: acceptancesPagination.current, page_size: acceptancesPagination.pageSize }
    const res = await request.get(`/admin/agreements/${currentAcceptanceId.value}/acceptances`, { params })
    const d = res.data?.data
    acceptancesData.value = d?.list || []
    acceptancesPagination.total = d?.total || 0
  } catch {
    // auto-displayed
  } finally {
    acceptancesLoading.value = false
  }
}

// === Preview ===
const showPreview = ref(false)
const previewHtml = computed(() => {
  if (!form.content) return ''
  return marked(form.content) as string
})

onMounted(fetchList)
</script>

<template>
  <div>
    <PageHeader title="用户协议" description="管理用户协议与隐私政策，支持版本管理和发布">
      <template #actions>
        <AButton type="primary" @click="openCreateModal">创建协议版本</AButton>
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <template #title>
        <div class="flex items-center justify-between w-full">
          <span>协议列表</span>
          <div class="flex gap-2">
            <ASelect v-model="codeFilter" :options="codeOptions" style="width: 130px" allow-clear placeholder="协议类型" @change="() => { pagination.current = 1; fetchList() }" />
            <ASelect v-model="statusFilter" :options="statusOptions" style="width: 120px" allow-clear placeholder="状态筛选" @change="() => { pagination.current = 1; fetchList() }" />
          </div>
        </div>
      </template>
      <ATable :columns="columns" :data="data" :loading="loading" row-key="id" :scroll="{ x: 1400 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchList" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchList() }" />
      </div>
    </ACard>

    <!-- Create / Edit Drawer -->
    <ADrawer v-model:visible="showModal" :title="editingId ? (isReadonly ? '查看协议' : '编辑协议') : '创建协议版本'" :width="720" :mask-closable="false" :footer="!isReadonly" unmount-on-close>
      <AForm :model="form" :auto-label-width="true" layout="vertical">
        <div class="flex gap-4">
          <AFormItem label="协议类型" required class="flex-1">
            <ASelect v-model="form.code" :disabled="!!editingId">
              <AOption value="terms">用户协议</AOption>
              <AOption value="privacy">隐私政策</AOption>
            </ASelect>
          </AFormItem>
          <AFormItem label="版本号" required class="flex-1">
            <AInput v-model="form.version" placeholder="例如: 1.0" :disabled="!!editingId" />
          </AFormItem>
        </div>
        <AFormItem label="标题" required>
          <AInput v-model="form.title" placeholder="请输入协议标题" :disabled="isReadonly" />
        </AFormItem>
        <AFormItem label="变更摘要">
          <AInput v-model="form.summary" placeholder="本版本的主要变更说明（可选）" :disabled="isReadonly" />
        </AFormItem>
        <AFormItem label="强制接受">
          <ASwitch v-model="form.force_accept" :disabled="isReadonly" />
          <span class="ml-2 text-gray-500 text-sm">开启后用户必须接受此协议才能继续使用</span>
        </AFormItem>
        <AFormItem label="协议正文" required>
          <div class="flex gap-2 mb-2">
            <AButton size="mini" type="text" @click="showPreview = !showPreview">
              {{ showPreview ? '编辑' : '预览' }}
            </AButton>
          </div>
          <ATextarea v-if="!showPreview" v-model="form.content" :auto-size="{ minRows: 12, maxRows: 30 }" placeholder="Markdown 格式的协议正文" :disabled="isReadonly" />
          <div v-else class="agreement-preview" v-html="previewHtml" />
        </AFormItem>
      </AForm>
      <template v-if="!isReadonly" #footer>
        <Space>
          <AButton @click="showModal = false">取消</AButton>
          <AButton type="primary" :loading="formLoading" @click="handleSubmit">确认</AButton>
        </Space>
      </template>
    </ADrawer>

    <!-- Acceptances Drawer -->
    <ADrawer v-model:visible="showAcceptances" :title="acceptancesTitle" :width="640" unmount-on-close>
      <ATable :columns="acceptanceColumns" :data="acceptancesData" :loading="acceptancesLoading" row-key="id" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="acceptancesPagination.current" :total="acceptancesPagination.total" :page-size="acceptancesPagination.pageSize" @change="fetchAcceptances" />
      </div>
    </ADrawer>
  </div>
</template>

<style scoped>
.agreement-preview {
  border: 1px solid var(--color-border-2);
  border-radius: 4px;
  padding: 16px;
  min-height: 200px;
  max-height: 500px;
  overflow-y: auto;
  background: var(--color-fill-1);
  line-height: 1.8;
  font-size: 14px;
}

.agreement-preview :deep(h1),
.agreement-preview :deep(h2),
.agreement-preview :deep(h3) {
  margin-top: 16px;
  margin-bottom: 8px;
}

.agreement-preview :deep(ul),
.agreement-preview :deep(ol) {
  padding-left: 20px;
}

.agreement-preview :deep(p) {
  margin-bottom: 8px;
}
</style>
