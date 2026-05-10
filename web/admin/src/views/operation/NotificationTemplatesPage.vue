<script setup lang="ts">
import { ref, reactive, onMounted, h } from 'vue'
import {
  Tag, Button, Space, Message,
} from '@arco-design/web-vue'
import type { TableColumnData } from '@arco-design/web-vue'
import PageHeader from '@/components/PageHeader.vue'
import request from '@/utils/request'

const loading = ref(false)
const templates = ref<any[]>([])
const pagination = reactive({ current: 1, pageSize: 20, total: 0, showPageSize: true, pageSizeOptions: [10, 20, 50] })

const channelFilter = ref<string | undefined>(undefined)
const channelOptions = [
  { label: '全部', value: '' },
  { label: '邮件', value: 'email' },
  { label: '站内信', value: 'in_app' },
  { label: '短信', value: 'sms' },
  { label: 'Webhook', value: 'webhook' },
]

const columns: TableColumnData[] = [
  { title: '模板编码', dataIndex: 'code', width: 180, render({ record }) { return h('span', { class: 'font-mono text-xs' }, record.code) } },
  { title: '主题', dataIndex: 'subject', width: 200 },
  { title: '渠道', dataIndex: 'channel', width: 100, render({ record }) {
    const map: Record<string, string> = { email: 'arcoblue', in_app: 'green', sms: 'orangered', webhook: 'purple' }
    const labelMap: Record<string, string> = { email: '邮件', in_app: '站内信', sms: '短信', webhook: 'Webhook' }
    return h(Tag, { color: map[record.channel] || undefined, size: 'small' }, () => labelMap[record.channel] || record.channel)
  }},
  { title: '状态', dataIndex: 'status', width: 90, render({ record }) {
    return h(Tag, { color: record.status === 'active' ? 'green' : undefined, size: 'small' }, () => record.status === 'active' ? '启用' : '禁用')
  }},
  { title: '更新时间', dataIndex: 'updated_at', width: 180 },
  {
    title: '操作', dataIndex: 'actions', width: 160, fixed: 'right',
    render({ record }) {
      return h(Space, { size: 'small' }, () => [
        h(Button, { size: 'small', type: 'text', onClick: () => openEditModal(record) }, () => '编辑'),
        h(Button, { size: 'small', type: 'text', onClick: () => openTestModal(record) }, () => '测试'),
      ])
    },
  },
]

async function fetchTemplates() {
  loading.value = true
  try {
    const params: any = { page: pagination.current, page_size: pagination.pageSize }
    if (channelFilter.value) params.channel = channelFilter.value
    const res = await request.get('/admin/notification/templates', { params })
    const data = res.data?.data
    templates.value = data?.data || []
    pagination.total = data?.total || 0
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    loading.value = false
  }
}

// Edit modal
const showEditModal = ref(false)
const editLoading = ref(false)
const editingCode = ref('')
const editForm = reactive({
  subject: '',
  body_template: '',
  channel: 'email',
})

function openEditModal(row: any) {
  editingCode.value = row.code
  editForm.subject = row.subject || ''
  editForm.body_template = row.body_template || ''
  editForm.channel = row.channel || 'email'
  showEditModal.value = true
}

async function handleEditSubmit(done: () => void) {
  if (!editForm.subject.trim()) {
    Message.warning('请输入主题')
    return
  }
  editLoading.value = true
  try {
    await request.put(`/admin/notification/templates/${editingCode.value}`, {
      subject: editForm.subject,
      body_template: editForm.body_template,
      channel: editForm.channel,
    })
    Message.success('更新成功')
    done()
    fetchTemplates()
  } catch {
    return false
  } finally {
    editLoading.value = false
  }
}

// Test modal
const showTestModal = ref(false)
const testLoading = ref(false)
const testingCode = ref('')
const testVariables = ref('{\n  \n}')
const testResult = ref('')
const testSubject = ref('')

function openTestModal(row: any) {
  testingCode.value = row.code
  testVariables.value = '{\n  \n}'
  testResult.value = ''
  testSubject.value = ''
  showTestModal.value = true
}

async function handleTestSubmit() {
  let vars: Record<string, any>
  try {
    vars = JSON.parse(testVariables.value)
  } catch {
    Message.warning('变量格式错误，请输入有效的 JSON')
    return
  }
  testLoading.value = true
  testResult.value = ''
  testSubject.value = ''
  try {
    const res = await request.post(`/admin/notification/templates/${testingCode.value}/test`, { variables: vars })
    testSubject.value = res.data?.data?.subject || ''
    testResult.value = res.data?.data?.rendered_body || res.data?.data?.body || ''
    Message.success('渲染成功')
  } catch {
    // error auto-displayed by Axios interceptor
  } finally {
    testLoading.value = false
  }
}

onMounted(fetchTemplates)
</script>

<template>
  <div>
    <PageHeader title="通知模板" description="管理各渠道的通知模板内容和格式">
      <template #actions>
        <ASelect v-model="channelFilter" :options="channelOptions" style="width: 120px" allow-clear placeholder="渠道筛选" @change="() => { pagination.current = 1; fetchTemplates() }" />
      </template>
    </PageHeader>

    <ACard :bordered="false">
      <ATable :columns="columns" :data="templates" :loading="loading" row-key="code" :scroll="{ x: 1000 }" :pagination="false" />
      <div class="mt-4 flex justify-end">
        <APagination v-model:current="pagination.current" v-model:page-size="pagination.pageSize" :total="pagination.total" :page-size-options="pagination.pageSizeOptions" show-page-size @change="fetchTemplates" @page-size-change="(s: number) => { pagination.pageSize = s; pagination.current = 1; fetchTemplates() }" />
      </div>
    </ACard>

    <!-- Edit Modal -->
    <AModal v-model:visible="showEditModal" title="编辑模板" :width="600" :mask-closable="false" :on-before-ok="handleEditSubmit" :ok-loading="editLoading">
      <AForm :model="editForm" :auto-label-width="true" layout="vertical">
        <AFormItem label="模板编码">
          <AInput :model-value="editingCode" disabled />
        </AFormItem>
        <AFormItem label="渠道">
          <ASelect v-model="editForm.channel" :options="channelOptions.filter(o => o.value)" />
        </AFormItem>
        <AFormItem label="主题" required>
          <AInput v-model="editForm.subject" placeholder="通知主题，支持变量如 {{.name}}" />
        </AFormItem>
        <AFormItem label="正文模板">
          <ATextarea v-model="editForm.body_template" :auto-size="{ minRows: 6, maxRows: 16 }" placeholder="通知正文模板，支持 Go template 语法" />
        </AFormItem>
      </AForm>
    </AModal>

    <!-- Test Modal -->
    <AModal v-model:visible="showTestModal" title="模板测试" :width="650" :footer="false">
      <AForm layout="vertical">
        <AFormItem label="模板编码">
          <AInput :model-value="testingCode" disabled />
        </AFormItem>
        <AFormItem label="变量（JSON 格式）">
          <ATextarea v-model="testVariables" :auto-size="{ minRows: 4, maxRows: 10 }" placeholder='{"name": "张三", "amount": "100.00"}' />
        </AFormItem>
        <AFormItem>
          <AButton type="primary" :loading="testLoading" @click="handleTestSubmit">渲染测试</AButton>
        </AFormItem>
      </AForm>
      <div v-if="testSubject || testResult" class="mt-2">
        <ADivider />
        <div v-if="testSubject" class="mb-3">
          <div class="text-xs text-gray-500 mb-1">渲染主题</div>
          <ACard :bordered="true" size="small">{{ testSubject }}</ACard>
        </div>
        <div v-if="testResult">
          <div class="text-xs text-gray-500 mb-1">渲染结果</div>
          <ACard :bordered="true" size="small">
            <pre class="whitespace-pre-wrap text-sm m-0">{{ testResult }}</pre>
          </ACard>
        </div>
      </div>
    </AModal>
  </div>
</template>
